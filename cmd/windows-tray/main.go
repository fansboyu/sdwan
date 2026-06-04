//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"englishlisten/sdwan/internal/version"

	"github.com/getlantern/systray"
)

const defaultControllerURL = "https://controller.englishlisten.cn"

//go:embed assets/logo.ico
var trayIcon []byte

type trayConfig struct {
	ControllerURL       string `json:"controller_url"`
	AutoSync            bool   `json:"auto_sync"`
	SyncIntervalSeconds int    `json:"sync_interval_seconds"`
}

type trayApp struct {
	mu          sync.Mutex
	configPath  string
	trayPath    string
	servicePath string
	config      trayConfig
	status      string
	syncRunning bool

	statusItem    *systray.MenuItem
	joinedItem    *systray.MenuItem
	connectedItem *systray.MenuItem
	autoItem      *systray.MenuItem
	startupItem   *systray.MenuItem
}

func main() {
	app := newTrayApp()
	systray.Run(app.onReady, app.onExit)
}

func newTrayApp() *trayApp {
	configDir := defaultConfigDir()
	return &trayApp{
		configPath:  filepath.Join(configDir, "agent.json"),
		trayPath:    filepath.Join(configDir, "tray.json"),
		servicePath: defaultServicePath(),
		config: trayConfig{
			ControllerURL:       defaultControllerURL,
			AutoSync:            true,
			SyncIntervalSeconds: 15,
		},
		status: "Starting",
	}
}

func (a *trayApp) onReady() {
	_ = a.loadConfig()

	systray.SetTitle("SD-WAN")
	systray.SetTooltip("SD-WAN Windows Client")
	systray.SetIcon(trayIcon)

	a.statusItem = systray.AddMenuItem("Status: Starting", "Current SD-WAN status")
	a.statusItem.Disable()
	a.joinedItem = systray.AddMenuItemCheckbox("Joined", "Device config exists", false)
	a.joinedItem.Disable()
	a.connectedItem = systray.AddMenuItemCheckbox("Connected", "Windows service is running", false)
	a.connectedItem.Disable()
	systray.AddSeparator()

	joinItem := systray.AddMenuItem("Join from Clipboard", "Copy Admin Token, then click this")
	connectItem := systray.AddMenuItem("Connect", "Start Windows service")
	syncItem := systray.AddMenuItem("Restart Service", "Restart Windows service")
	disconnectItem := systray.AddMenuItem("Disconnect", "Stop Windows service")
	a.autoItem = systray.AddMenuItemCheckbox("Auto Start", "Start service when tray opens", a.config.AutoSync)
	systray.AddSeparator()

	editItem := systray.AddMenuItem("Edit Settings", "Open tray.json in Notepad")
	folderItem := systray.AddMenuItem("Open Config Folder", "Open C:\\ProgramData\\sdwan")
	a.startupItem = systray.AddMenuItemCheckbox("Start with Windows", "Install Startup shortcut", a.isStartupInstalled())
	aboutItem := systray.AddMenuItem("About", "Show version")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Exit", "Exit tray")

	a.setStatus("Ready")
	if _, err := os.Stat(a.configPath); errors.Is(err, os.ErrNotExist) {
		a.setStatus("Not joined")
	}

	if a.config.AutoSync {
		go a.startService()
	}
	a.refreshIndicators()
	go a.refreshIndicatorsLoop()

	go func() {
		for {
			select {
			case <-joinItem.ClickedCh:
				go a.joinFromClipboard()
			case <-connectItem.ClickedCh:
				go a.startService()
			case <-syncItem.ClickedCh:
				go a.restartService()
			case <-disconnectItem.ClickedCh:
				go a.disconnect()
			case <-a.autoItem.ClickedCh:
				a.toggleAutoSync()
			case <-editItem.ClickedCh:
				go a.openSettings()
			case <-folderItem.ClickedCh:
				go a.openConfigFolder()
			case <-a.startupItem.ClickedCh:
				go a.toggleStartup()
			case <-aboutItem.ClickedCh:
				a.setStatus("sdwan-tray " + version.Version + " " + runtime.GOARCH)
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func (a *trayApp) onExit() {}

func (a *trayApp) joinFromClipboard() {
	token, err := readClipboard()
	if err != nil {
		a.setStatus("Clipboard read failed")
		return
	}
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		a.setStatus("Clipboard has no token")
		return
	}
	if err := validateControllerURL(a.config.ControllerURL); err != nil {
		a.setStatus("Bad controller URL")
		return
	}

	a.setStatus("Joining")
	output, err := a.runServiceCommand("register", "--config", a.configPath, "--controller", a.config.ControllerURL, "--admin-token", token)
	if err != nil {
		a.setStatus("Join failed: " + shortError(err))
		return
	}
	a.setStatus("Joined")
	_ = output
	a.refreshIndicators()
	go a.startService()
}

func (a *trayApp) startService() {
	if !a.tryStartSync() {
		return
	}
	defer a.finishSync()

	if _, err := os.Stat(a.configPath); err != nil {
		a.setStatus("Not joined")
		return
	}
	if _, err := a.runServiceCommand("install-service", "--config", a.configPath, "--auto-start=true"); err != nil && !strings.Contains(err.Error(), "already installed") {
		a.setStatus("Install failed: " + shortError(err))
		return
	}
	if _, err := a.runServiceCommand("start-service"); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already running") {
		a.setStatus("Start failed: " + shortError(err))
		return
	}
	a.setStatus("Connected")
	a.refreshIndicators()
}

func (a *trayApp) restartService() {
	_, _ = a.runServiceCommand("stop-service")
	go a.startService()
}

func (a *trayApp) runServiceCommand(args ...string) (string, error) {
	cmd := exec.Command(a.servicePath, args...)
	cmd.SysProcAttr = hiddenProcessAttrs()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func (a *trayApp) disconnect() {
	if _, err := a.runServiceCommand("stop-service"); err != nil {
		a.setStatus("Disconnect failed")
		return
	}
	a.setStatus("Disconnected")
	a.refreshIndicators()
}

func defaultConfigDir() string {
	if programData := os.Getenv("ProgramData"); programData != "" {
		return filepath.Join(programData, "sdwan")
	}
	return filepath.Join(`C:\ProgramData`, "sdwan")
}

func defaultServicePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "sdwan-service.exe"
	}
	return filepath.Join(filepath.Dir(exe), "sdwan-service.exe")
}

func hiddenProcessAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

func (a *trayApp) openSettings() {
	if err := a.ensureTrayConfig(); err != nil {
		a.setStatus("Settings failed")
		return
	}
	_ = exec.Command("notepad.exe", a.trayPath).Start()
}

func (a *trayApp) openConfigFolder() {
	_ = os.MkdirAll(filepath.Dir(a.configPath), 0o700)
	_ = exec.Command("explorer.exe", filepath.Dir(a.configPath)).Start()
}

func (a *trayApp) startupShortcutPath() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`, "sdwan-tray.cmd")
}

func (a *trayApp) isStartupInstalled() bool {
	if a.startupShortcutPath() == "" {
		return false
	}
	_, err := os.Stat(a.startupShortcutPath())
	return err == nil
}

func (a *trayApp) toggleStartup() {
	if a.isStartupInstalled() {
		if err := os.Remove(a.startupShortcutPath()); err != nil {
			a.setStatus("Startup remove failed")
			a.refreshIndicators()
			return
		}
		a.setStatus("Startup removed")
		a.refreshIndicators()
		return
	}
	a.installStartup()
}

func (a *trayApp) installStartup() {
	exe, err := os.Executable()
	if err != nil {
		a.setStatus("Startup failed")
		a.refreshIndicators()
		return
	}
	cmdPath := a.startupShortcutPath()
	startupDir := filepath.Dir(cmdPath)
	if startupDir == "" {
		a.setStatus("Startup path failed")
		a.refreshIndicators()
		return
	}
	if err := os.MkdirAll(startupDir, 0o700); err != nil {
		a.setStatus("Startup failed")
		a.refreshIndicators()
		return
	}
	content := fmt.Sprintf("@echo off\r\nstart \"\" \"%s\"\r\n", exe)
	if err := os.WriteFile(cmdPath, []byte(content), 0o600); err != nil {
		a.setStatus("Startup failed")
		a.refreshIndicators()
		return
	}
	a.setStatus("Startup installed")
	a.refreshIndicators()
}

func (a *trayApp) toggleAutoSync() {
	a.mu.Lock()
	a.config.AutoSync = !a.config.AutoSync
	enabled := a.config.AutoSync
	a.mu.Unlock()

	if enabled {
		a.autoItem.Check()
	} else {
		a.autoItem.Uncheck()
	}
	_ = a.saveConfig()
	a.setStatus(fmt.Sprintf("Auto Start: %v", enabled))
	a.refreshIndicators()
}

func (a *trayApp) refreshIndicatorsLoop() {
	for {
		a.refreshIndicators()
		sleepSeconds := a.config.SyncIntervalSeconds
		if sleepSeconds <= 0 {
			sleepSeconds = 15
		}
		if sleepSeconds > 60 {
			sleepSeconds = 60
		}
		select {
		case <-time.After(time.Duration(sleepSeconds) * time.Second):
		}
	}
}

func (a *trayApp) refreshIndicators() {
	joined := fileExists(a.configPath)
	connected := a.isServiceRunning()
	startup := a.isStartupInstalled()

	setMenuChecked(a.joinedItem, joined)
	setMenuChecked(a.connectedItem, connected)
	setMenuChecked(a.startupItem, startup)
}

func (a *trayApp) isServiceRunning() bool {
	cmd := exec.Command("sc.exe", "query", "SDWANService")
	cmd.SysProcAttr = hiddenProcessAttrs()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToUpper(string(output)), "RUNNING")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func setMenuChecked(item *systray.MenuItem, checked bool) {
	if item == nil {
		return
	}
	if checked {
		item.Check()
		return
	}
	item.Uncheck()
}

func (a *trayApp) tryStartSync() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.syncRunning {
		return false
	}
	a.syncRunning = true
	return true
}

func (a *trayApp) finishSync() {
	a.mu.Lock()
	a.syncRunning = false
	a.mu.Unlock()
}

func (a *trayApp) setStatus(status string) {
	a.mu.Lock()
	a.status = status
	a.mu.Unlock()

	title := "Status: " + status
	if len(title) > 120 {
		title = title[:120]
	}
	if a.statusItem != nil {
		a.statusItem.SetTitle(title)
	}
	tooltip := "SD-WAN - " + status
	if len(tooltip) > 120 {
		tooltip = tooltip[:120]
	}
	systray.SetTooltip(tooltip)
}

func (a *trayApp) loadConfig() error {
	if err := a.ensureTrayConfig(); err != nil {
		return err
	}
	data, err := os.ReadFile(a.trayPath)
	if err != nil {
		return err
	}
	var cfg trayConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.ControllerURL) != "" {
		a.config.ControllerURL = strings.TrimSpace(cfg.ControllerURL)
	}
	a.config.AutoSync = cfg.AutoSync
	if cfg.SyncIntervalSeconds > 0 {
		a.config.SyncIntervalSeconds = cfg.SyncIntervalSeconds
	}
	return nil
}

func (a *trayApp) saveConfig() error {
	if err := os.MkdirAll(filepath.Dir(a.trayPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.trayPath, data, 0o600)
}

func (a *trayApp) ensureTrayConfig() error {
	if _, err := os.Stat(a.trayPath); err == nil {
		return nil
	}
	return a.saveConfig()
}

func readClipboard() (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", "Get-Clipboard")
	cmd.SysProcAttr = hiddenProcessAttrs()
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func validateControllerURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("controller URL must start with http or https")
	}
	if parsed.Host == "" {
		return errors.New("controller URL host is required")
	}
	return nil
}

func shortError(err error) string {
	text := strings.TrimSpace(err.Error())
	if len(text) <= 64 {
		return text
	}
	return text[:64]
}
