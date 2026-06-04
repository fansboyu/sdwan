Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$ErrorActionPreference = "Stop"

$agentExe = Join-Path $PSScriptRoot "sdwan-agent.exe"
if (!(Test-Path $agentExe)) {
    $agentExe = "sdwan-agent.exe"
}

$programData = $env:ProgramData
if ([string]::IsNullOrWhiteSpace($programData)) {
    $programData = "C:\ProgramData"
}
$configDir = Join-Path $programData "sdwan"
$configPath = Join-Path $configDir "agent.json"
$wgConfigPath = Join-Path $configDir "sdwan0.conf"
$statePath = Join-Path $configDir "tray-state.json"

$script:controllerUrl = "https://controller.englishlisten.cn"
$script:adminToken = ""
$script:lastStatus = "Idle"
$script:syncRunning = $false
$script:autoSync = $true
$script:syncIntervalSeconds = 15

function Test-IsAdmin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Save-State {
    New-Item -ItemType Directory -Force -Path $configDir | Out-Null
    @{
        controller_url = $script:controllerUrl
        auto_sync = $script:autoSync
        sync_interval_seconds = $script:syncIntervalSeconds
    } | ConvertTo-Json | Set-Content -Encoding UTF8 -Path $statePath
}

function Load-State {
    if (!(Test-Path $statePath)) {
        return
    }
    try {
        $state = Get-Content -Raw -Path $statePath | ConvertFrom-Json
        if (![string]::IsNullOrWhiteSpace($state.controller_url)) {
            $script:controllerUrl = $state.controller_url
        }
        if ($null -ne $state.auto_sync) {
            $script:autoSync = [bool]$state.auto_sync
        }
        if ($state.sync_interval_seconds -gt 0) {
            $script:syncIntervalSeconds = [int]$state.sync_interval_seconds
        }
    } catch {
        $script:lastStatus = "Failed to load tray state: $($_.Exception.Message)"
    }
}

function Invoke-Agent {
    param([string[]]$Arguments)

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $agentExe
    foreach ($arg in $Arguments) {
        [void]$psi.ArgumentList.Add($arg)
    }
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true

    $process = [System.Diagnostics.Process]::Start($psi)
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()

    if ($process.ExitCode -ne 0) {
        throw "sdwan-agent exited with code $($process.ExitCode)`r`n$stderr`r`n$stdout"
    }
    return $stdout.Trim()
}

function Set-TrayStatus {
    param([string]$Text)
    $script:lastStatus = $Text
    $notify.Text = ("SD-WAN - " + $Text)
    if ($notify.Text.Length -gt 63) {
        $notify.Text = $notify.Text.Substring(0, 63)
    }
}

function Show-Balloon {
    param([string]$Title, [string]$Text)
    $notify.BalloonTipTitle = $Title
    $notify.BalloonTipText = $Text
    $notify.ShowBalloonTip(3000)
}

function Invoke-SyncOnce {
    param([bool]$ShowErrors = $false)

    if ($script:syncRunning) {
        return
    }
    if (!(Test-Path $configPath)) {
        Set-TrayStatus "Not joined"
        return
    }
    if (!(Test-IsAdmin)) {
        Set-TrayStatus "Run as Administrator"
        if ($ShowErrors) {
            Show-Balloon "SD-WAN" "Please run tray as Administrator."
        }
        return
    }

    $script:syncRunning = $true
    try {
        Set-TrayStatus "Syncing"
        Invoke-Agent @("daemon", "--once", "--config", $configPath, "--wg-config", $wgConfigPath) | Out-Null
        Set-TrayStatus "Connected"
    } catch {
        Set-TrayStatus "Sync failed"
        if ($ShowErrors) {
            Show-Balloon "SD-WAN sync failed" $_.Exception.Message
        }
    } finally {
        $script:syncRunning = $false
    }
}

function Show-SettingsWindow {
    $form = New-Object System.Windows.Forms.Form
    $form.Text = "SD-WAN Settings"
    $form.Size = New-Object System.Drawing.Size(720, 430)
    $form.StartPosition = "CenterScreen"

    $labelController = New-Object System.Windows.Forms.Label
    $labelController.Text = "Controller"
    $labelController.Location = New-Object System.Drawing.Point(20, 24)
    $labelController.Size = New-Object System.Drawing.Size(120, 24)
    $form.Controls.Add($labelController)

    $controllerBox = New-Object System.Windows.Forms.TextBox
    $controllerBox.Text = $script:controllerUrl
    $controllerBox.Location = New-Object System.Drawing.Point(150, 22)
    $controllerBox.Size = New-Object System.Drawing.Size(520, 24)
    $form.Controls.Add($controllerBox)

    $labelToken = New-Object System.Windows.Forms.Label
    $labelToken.Text = "Admin Token"
    $labelToken.Location = New-Object System.Drawing.Point(20, 60)
    $labelToken.Size = New-Object System.Drawing.Size(120, 24)
    $form.Controls.Add($labelToken)

    $tokenBox = New-Object System.Windows.Forms.TextBox
    $tokenBox.Location = New-Object System.Drawing.Point(150, 58)
    $tokenBox.Size = New-Object System.Drawing.Size(520, 24)
    $form.Controls.Add($tokenBox)

    $autoSyncBox = New-Object System.Windows.Forms.CheckBox
    $autoSyncBox.Text = "Auto sync in tray"
    $autoSyncBox.Checked = $script:autoSync
    $autoSyncBox.Location = New-Object System.Drawing.Point(150, 94)
    $autoSyncBox.Size = New-Object System.Drawing.Size(180, 24)
    $form.Controls.Add($autoSyncBox)

    $labelInterval = New-Object System.Windows.Forms.Label
    $labelInterval.Text = "Sync interval"
    $labelInterval.Location = New-Object System.Drawing.Point(360, 96)
    $labelInterval.Size = New-Object System.Drawing.Size(90, 24)
    $form.Controls.Add($labelInterval)

    $intervalBox = New-Object System.Windows.Forms.NumericUpDown
    $intervalBox.Minimum = 5
    $intervalBox.Maximum = 3600
    $intervalBox.Value = $script:syncIntervalSeconds
    $intervalBox.Location = New-Object System.Drawing.Point(455, 94)
    $intervalBox.Size = New-Object System.Drawing.Size(80, 24)
    $form.Controls.Add($intervalBox)

    $secondsLabel = New-Object System.Windows.Forms.Label
    $secondsLabel.Text = "seconds"
    $secondsLabel.Location = New-Object System.Drawing.Point(545, 96)
    $secondsLabel.Size = New-Object System.Drawing.Size(80, 24)
    $form.Controls.Add($secondsLabel)

    $joinButton = New-Object System.Windows.Forms.Button
    $joinButton.Text = "Join"
    $joinButton.Location = New-Object System.Drawing.Point(20, 135)
    $joinButton.Size = New-Object System.Drawing.Size(110, 34)
    $form.Controls.Add($joinButton)

    $connectButton = New-Object System.Windows.Forms.Button
    $connectButton.Text = "Connect"
    $connectButton.Location = New-Object System.Drawing.Point(145, 135)
    $connectButton.Size = New-Object System.Drawing.Size(110, 34)
    $form.Controls.Add($connectButton)

    $disconnectButton = New-Object System.Windows.Forms.Button
    $disconnectButton.Text = "Disconnect"
    $disconnectButton.Location = New-Object System.Drawing.Point(270, 135)
    $disconnectButton.Size = New-Object System.Drawing.Size(110, 34)
    $form.Controls.Add($disconnectButton)

    $saveButton = New-Object System.Windows.Forms.Button
    $saveButton.Text = "Save"
    $saveButton.Location = New-Object System.Drawing.Point(395, 135)
    $saveButton.Size = New-Object System.Drawing.Size(110, 34)
    $form.Controls.Add($saveButton)

    $logBox = New-Object System.Windows.Forms.TextBox
    $logBox.Multiline = $true
    $logBox.ScrollBars = "Vertical"
    $logBox.ReadOnly = $true
    $logBox.Location = New-Object System.Drawing.Point(20, 185)
    $logBox.Size = New-Object System.Drawing.Size(650, 160)
    $form.Controls.Add($logBox)

    function Append-SettingsLog {
        param([string]$Text)
        $timestamp = Get-Date -Format "HH:mm:ss"
        $logBox.AppendText("[$timestamp] $Text`r`n")
    }

    $joinButton.Add_Click({
        try {
            if ([string]::IsNullOrWhiteSpace($tokenBox.Text)) {
                throw "Admin Token is required."
            }
            $script:controllerUrl = $controllerBox.Text.Trim()
            $script:autoSync = $autoSyncBox.Checked
            $script:syncIntervalSeconds = [int]$intervalBox.Value
            Save-State
            New-Item -ItemType Directory -Force -Path $configDir | Out-Null
            $result = Invoke-Agent @(
                "register",
                "--config", $configPath,
                "--controller", $script:controllerUrl,
                "--admin-token", $tokenBox.Text.Trim()
            )
            Append-SettingsLog "Joined network."
            Append-SettingsLog $result
            Show-Balloon "SD-WAN" "Joined network."
        } catch {
            Append-SettingsLog "Join failed: $($_.Exception.Message)"
        }
    })

    $connectButton.Add_Click({
        try {
            Invoke-SyncOnce -ShowErrors $true
            Append-SettingsLog $script:lastStatus
        } catch {
            Append-SettingsLog "Connect failed: $($_.Exception.Message)"
        }
    })

    $disconnectButton.Add_Click({
        try {
            if (!(Test-IsAdmin)) {
                throw "Please run tray as Administrator."
            }
            Invoke-Agent @("down", "--config", $configPath) | Out-Null
            Set-TrayStatus "Disconnected"
            Append-SettingsLog "Disconnected."
        } catch {
            Append-SettingsLog "Disconnect failed: $($_.Exception.Message)"
        }
    })

    $saveButton.Add_Click({
        $script:controllerUrl = $controllerBox.Text.Trim()
        $script:autoSync = $autoSyncBox.Checked
        $script:syncIntervalSeconds = [int]$intervalBox.Value
        $timer.Interval = $script:syncIntervalSeconds * 1000
        Save-State
        Append-SettingsLog "Saved."
    })

    Append-SettingsLog "Config: $configPath"
    Append-SettingsLog "Status: $script:lastStatus"
    [void]$form.ShowDialog()
}

function Install-StartupShortcut {
    $startup = [Environment]::GetFolderPath("Startup")
    $shortcutPath = Join-Path $startup "SD-WAN Tray.lnk"
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = "powershell.exe"
    $shortcut.Arguments = "-ExecutionPolicy Bypass -WindowStyle Hidden -File `"$PSCommandPath`""
    $shortcut.WorkingDirectory = $PSScriptRoot
    $shortcut.IconLocation = "$env:SystemRoot\System32\shell32.dll,18"
    $shortcut.Save()
    Show-Balloon "SD-WAN" "Startup shortcut installed."
}

Load-State

$notify = New-Object System.Windows.Forms.NotifyIcon
$notify.Icon = [System.Drawing.SystemIcons]::Application
$notify.Visible = $true

$menu = New-Object System.Windows.Forms.ContextMenuStrip
$openItem = $menu.Items.Add("Open Settings")
$syncItem = $menu.Items.Add("Sync Now")
$connectItem = $menu.Items.Add("Connect")
$disconnectItem = $menu.Items.Add("Disconnect")
$statusItem = $menu.Items.Add("Show Status")
$startupItem = $menu.Items.Add("Start with Windows")
$exitItem = $menu.Items.Add("Exit")

$openItem.add_Click({ Show-SettingsWindow })
$syncItem.add_Click({ Invoke-SyncOnce -ShowErrors $true })
$connectItem.add_Click({ Invoke-SyncOnce -ShowErrors $true })
$disconnectItem.add_Click({
    try {
        if (!(Test-IsAdmin)) {
            throw "Please run tray as Administrator."
        }
        Invoke-Agent @("down", "--config", $configPath) | Out-Null
        Set-TrayStatus "Disconnected"
        Show-Balloon "SD-WAN" "Disconnected."
    } catch {
        Show-Balloon "SD-WAN disconnect failed" $_.Exception.Message
    }
})
$statusItem.add_Click({ Show-Balloon "SD-WAN Status" $script:lastStatus })
$startupItem.add_Click({ Install-StartupShortcut })
$exitItem.add_Click({
    $timer.Stop()
    $notify.Visible = $false
    $notify.Dispose()
    [System.Windows.Forms.Application]::Exit()
})

$notify.ContextMenuStrip = $menu
$notify.add_DoubleClick({ Show-SettingsWindow })

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = $script:syncIntervalSeconds * 1000
$timer.Add_Tick({
    if ($script:autoSync) {
        Invoke-SyncOnce -ShowErrors $false
    }
})
$timer.Start()

Set-TrayStatus "Starting"
if (!(Test-IsAdmin)) {
    Set-TrayStatus "Run as Administrator"
    Show-Balloon "SD-WAN" "Run this tray as Administrator to manage WireGuard."
} elseif (Test-Path $configPath) {
    Invoke-SyncOnce -ShowErrors $false
} else {
    Set-TrayStatus "Not joined"
}

[System.Windows.Forms.Application]::Run()
