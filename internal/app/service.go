package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"time"

	"englishlisten/sdwan/internal/config"
	"englishlisten/sdwan/internal/storage"
	"englishlisten/sdwan/internal/storage/sqlc"
	"englishlisten/sdwan/internal/version"
	"github.com/jackc/pgx/v5"
	"github.com/segmentio/ksuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	userCIDRPrefix = 24
	globalBaseCIDR = "100.64.0.0/10"
	maxNodeLimit   = 254
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrUpgradeRequired = errors.New("upgrade required")

type Service struct {
	store *storage.Store
	cfg   config.Config
}

func NewService(store *storage.Store, cfg config.Config) *Service {
	return &Service{store: store, cfg: cfg}
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	User      sqlc.User `json:"user"`
	AdminUser sqlc.User `json:"admin_user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) RegisterAdmin(ctx context.Context, req AuthRequest) (AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return AuthResponse{}, err
	}
	if len(req.Password) < 8 {
		return AuthResponse{}, errors.New("password must be at least 8 characters")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResponse{}, err
	}
	var user sqlc.User
	err = s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockOverlayIPAllocator(ctx); err != nil {
			return err
		}
		cidr, err := nextUserCIDR(ctx, q)
		if err != nil {
			return err
		}
		user, err = q.CreateUser(ctx, sqlc.CreateUserParams{
			ID:           "usr_" + ksuid.New().String(),
			Email:        email,
			PasswordHash: string(passwordHash),
			OverlayCidr:  cidr,
			MaxDevices:   effectiveDeviceLimit(s.cfg.DefaultMaxDevices),
		})
		return err
	})
	if err != nil {
		return AuthResponse{}, err
	}
	return s.createAdminSession(ctx, user)
}

func (s *Service) LoginAdmin(ctx context.Context, req AuthRequest) (AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return AuthResponse{}, err
	}
	user, err := s.store.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResponse{}, ErrUnauthorized
		}
		return AuthResponse{}, err
	}
	if user.Status != "active" {
		return AuthResponse{}, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return AuthResponse{}, ErrUnauthorized
	}
	return s.createAdminSession(ctx, user)
}

func (s *Service) AdminFromToken(ctx context.Context, bearer string) (sqlc.User, error) {
	token := strings.TrimPrefix(strings.TrimSpace(bearer), "Bearer ")
	if token == "" {
		return sqlc.User{}, ErrUnauthorized
	}
	user, err := s.store.Queries.GetUserBySessionTokenHash(ctx, tokenHash(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, ErrUnauthorized
		}
		return sqlc.User{}, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) createAdminSession(ctx context.Context, user sqlc.User) (AuthResponse, error) {
	token, err := randomToken("sdwan_admin")
	if err != nil {
		return AuthResponse{}, err
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	_, err = s.store.Queries.CreateAdminSession(ctx, sqlc.CreateAdminSessionParams{
		ID:        "ases_" + ksuid.New().String(),
		UserID:    user.ID,
		TokenHash: tokenHash(token),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return AuthResponse{}, err
	}
	user.PasswordHash = ""
	return AuthResponse{User: user, AdminUser: user, Token: token, ExpiresAt: expiresAt}, nil
}

type AccountSummary struct {
	User         sqlc.User           `json:"user"`
	DeviceCount  int32               `json:"device_count"`
	Plans        []sqlc.Plan         `json:"plans"`
	SubnetRoutes []sqlc.SubnetRoute  `json:"subnet_routes"`
	Relays       []sqlc.Relay        `json:"relays"`
	Capabilities AccountCapabilities `json:"capabilities"`
}

type AccountCapabilities struct {
	EnableSubnet    bool `json:"enable_subnet"`
	EnableSelfRelay bool `json:"enable_self_relay"`
}

func (s *Service) AccountSummary(ctx context.Context, user sqlc.User) (AccountSummary, error) {
	count, err := s.store.Queries.CountActiveDevicesByUser(ctx, user.ID)
	if err != nil {
		return AccountSummary{}, err
	}
	plans, err := s.store.Queries.ListPlans(ctx)
	if err != nil {
		return AccountSummary{}, err
	}
	subnets, err := s.store.Queries.ListSubnetRoutesByUser(ctx, user.ID)
	if err != nil {
		return AccountSummary{}, err
	}
	relays, err := s.store.Queries.ListRelaysByUser(ctx, user.ID)
	if err != nil {
		return AccountSummary{}, err
	}
	user.PasswordHash = ""
	return AccountSummary{
		User:         user,
		DeviceCount:  count,
		Plans:        plans,
		SubnetRoutes: subnets,
		Relays:       relays,
		Capabilities: capabilitiesForPlan(user.PlanCode),
	}, nil
}

func (s *Service) ListPlans(ctx context.Context) ([]sqlc.Plan, error) {
	return s.store.Queries.ListPlans(ctx)
}

type RegisterDeviceRequest struct {
	AdminToken    string `json:"admin_token"`
	Hostname      string `json:"hostname"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	OSVersion     string `json:"os_version"`
	PublicKey     string `json:"public_key"`
	ClientVersion string `json:"client_version"`
}

type RegisterDeviceResponse struct {
	DeviceID      string `json:"device_id"`
	DeviceToken   string `json:"device_token"`
	VirtualIP     string `json:"virtual_ip"`
	ControlURL    string `json:"control_url"`
	NetmapVersion int64  `json:"netmap_version"`
}

func (s *Service) RegisterDevice(ctx context.Context, req RegisterDeviceRequest) (RegisterDeviceResponse, error) {
	user, err := s.AdminFromToken(ctx, req.AdminToken)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	count, err := s.store.Queries.CountActiveDevicesByUser(ctx, user.ID)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	maxDevices := effectiveDeviceLimit(user.MaxDevices)
	if count >= maxDevices {
		return RegisterDeviceResponse{}, ErrUpgradeRequired
	}
	devices, err := s.store.Queries.ListDevicesByUser(ctx, user.ID)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	virtualIP, err := allocateDeviceIP(user.OverlayCidr, devices)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	deviceToken, err := randomToken("sdwan_device")
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	device, err := s.store.Queries.CreateDevice(ctx, sqlc.CreateDeviceParams{
		ID:              "dev_" + ksuid.New().String(),
		UserID:          user.ID,
		Hostname:        defaultString(req.Hostname, "unnamed"),
		OS:              defaultString(req.OS, "unknown"),
		Arch:            req.Arch,
		PublicKey:       strings.TrimSpace(req.PublicKey),
		VirtualIP:       virtualIP,
		DeviceTokenHash: tokenHash(deviceToken),
		ClientVersion:   defaultString(req.ClientVersion, "unknown"),
		OSVersion:       req.OSVersion,
	})
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	if err := s.store.Queries.BumpNetmapVersion(ctx, user.ID); err != nil {
		return RegisterDeviceResponse{}, err
	}
	user, _ = s.store.Queries.GetUser(ctx, user.ID)
	return RegisterDeviceResponse{
		DeviceID:      device.ID,
		DeviceToken:   deviceToken,
		VirtualIP:     device.VirtualIP,
		ControlURL:    s.cfg.ControllerURL,
		NetmapVersion: user.NetmapVersion,
	}, nil
}

type DeviceDetail struct {
	Device    sqlc.Device           `json:"device"`
	User      sqlc.User             `json:"user"`
	Endpoints []sqlc.DeviceEndpoint `json:"endpoints"`
}

func (s *Service) ListDevices(ctx context.Context, user sqlc.User) ([]sqlc.Device, error) {
	return s.store.Queries.ListDevicesByUser(ctx, user.ID)
}

func (s *Service) GetDeviceDetail(ctx context.Context, user sqlc.User, deviceID string) (DeviceDetail, error) {
	device, err := s.store.Queries.GetDevice(ctx, deviceID)
	if err != nil {
		return DeviceDetail{}, err
	}
	if device.UserID != user.ID {
		return DeviceDetail{}, ErrUnauthorized
	}
	endpoints, err := s.store.Queries.ListEndpointsByDevice(ctx, device.ID)
	if err != nil {
		return DeviceDetail{}, err
	}
	user.PasswordHash = ""
	return DeviceDetail{Device: device, User: user, Endpoints: endpoints}, nil
}

type EndpointReport struct {
	Type    string `json:"type"`
	Address string `json:"addr"`
	Source  string `json:"source"`
	RttMs   *int32 `json:"rtt_ms,omitempty"`
}

type PollRequest struct {
	CurrentNetmapVersion int64            `json:"current_netmap_version"`
	ClientVersion        string           `json:"client_version"`
	OSVersion            string           `json:"os_version"`
	Endpoints            []EndpointReport `json:"endpoints"`
}

type PollResponse struct {
	ServerTime          time.Time       `json:"server_time"`
	DeviceStatus        string          `json:"device_status"`
	NetmapVersion       int64           `json:"netmap_version"`
	NetmapChanged       bool            `json:"netmap_changed"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
	Upgrade             UpgradeResponse `json:"upgrade"`
}

type UpgradeResponse struct {
	Required      bool   `json:"required"`
	Recommended   bool   `json:"recommended"`
	LatestVersion string `json:"latest_version"`
	Message       string `json:"message"`
}

func (s *Service) Poll(ctx context.Context, token string, req PollRequest) (PollResponse, error) {
	device, err := s.deviceFromToken(ctx, token)
	if err != nil {
		return PollResponse{}, err
	}
	if err := s.store.Queries.UpdateDeviceHeartbeat(ctx, device.ID, defaultString(req.ClientVersion, device.ClientVersion), req.OSVersion); err != nil {
		return PollResponse{}, err
	}
	endpointChanged := false
	for _, endpoint := range req.Endpoints {
		if strings.TrimSpace(endpoint.Address) == "" {
			continue
		}
		changed, err := s.store.Queries.UpsertDeviceEndpoint(ctx, sqlc.UpsertDeviceEndpointParams{
			ID:           "dep_" + ksuid.New().String(),
			DeviceID:     device.ID,
			EndpointType: defaultString(endpoint.Type, "unknown"),
			Address:      endpoint.Address,
			Source:       endpoint.Source,
			RttMs:        endpoint.RttMs,
		})
		if err != nil {
			return PollResponse{}, err
		}
		endpointChanged = endpointChanged || changed
	}
	if endpointChanged {
		if err := s.store.Queries.BumpNetmapVersion(ctx, device.UserID); err != nil {
			return PollResponse{}, err
		}
	}
	user, err := s.store.Queries.GetUser(ctx, device.UserID)
	if err != nil {
		return PollResponse{}, err
	}
	return PollResponse{
		ServerTime:          time.Now().UTC(),
		DeviceStatus:        device.Status,
		NetmapVersion:       user.NetmapVersion,
		NetmapChanged:       req.CurrentNetmapVersion != user.NetmapVersion,
		PollIntervalSeconds: int(s.cfg.DefaultPollInterval.Seconds()),
		Upgrade: UpgradeResponse{
			Required:      false,
			Recommended:   req.ClientVersion != "" && req.ClientVersion != s.cfg.LatestClientVersion,
			LatestVersion: s.cfg.LatestClientVersion,
			Message:       "client update available",
		},
	}, nil
}

type Netmap struct {
	Version     int64         `json:"version"`
	Self        NetmapSelf    `json:"self"`
	Peers       []NetmapPeer  `json:"peers"`
	STUNServers []string      `json:"stun_servers"`
	Relays      []interface{} `json:"relays"`
}

type NetmapSelf struct {
	DeviceID  string `json:"device_id"`
	Hostname  string `json:"hostname"`
	VirtualIP string `json:"virtual_ip"`
	PublicKey string `json:"public_key"`
}

type NetmapPeer struct {
	DeviceID            string   `json:"device_id"`
	Hostname            string   `json:"hostname"`
	VirtualIP           string   `json:"virtual_ip"`
	PublicKey           string   `json:"public_key"`
	AllowedIPs          []string `json:"allowed_ips"`
	Endpoints           []string `json:"endpoints"`
	PersistentKeepalive int      `json:"persistent_keepalive"`
}

func (s *Service) Netmap(ctx context.Context, token string) (Netmap, error) {
	self, err := s.deviceFromToken(ctx, token)
	if err != nil {
		return Netmap{}, err
	}
	user, err := s.store.Queries.GetUser(ctx, self.UserID)
	if err != nil {
		return Netmap{}, err
	}
	devices, err := s.store.Queries.ListDevicesByUser(ctx, self.UserID)
	if err != nil {
		return Netmap{}, err
	}
	endpoints, err := s.store.Queries.ListEndpointsByUser(ctx, self.UserID)
	if err != nil {
		return Netmap{}, err
	}
	endpointsByDevice := map[string][]string{}
	for _, endpoint := range endpoints {
		endpointsByDevice[endpoint.DeviceID] = append(endpointsByDevice[endpoint.DeviceID], endpoint.Address)
	}

	peers := make([]NetmapPeer, 0, len(devices)-1)
	for _, d := range devices {
		if d.ID == self.ID || d.Status != "active" {
			continue
		}
		peerEndpoints := endpointsByDevice[d.ID]
		sort.Strings(peerEndpoints)
		peers = append(peers, NetmapPeer{
			DeviceID:            d.ID,
			Hostname:            d.Hostname,
			VirtualIP:           d.VirtualIP,
			PublicKey:           d.PublicKey,
			AllowedIPs:          []string{d.VirtualIP + "/32"},
			Endpoints:           peerEndpoints,
			PersistentKeepalive: 25,
		})
	}
	return Netmap{
		Version: user.NetmapVersion,
		Self: NetmapSelf{
			DeviceID:  self.ID,
			Hostname:  self.Hostname,
			VirtualIP: self.VirtualIP,
			PublicKey: self.PublicKey,
		},
		Peers:       peers,
		STUNServers: s.cfg.STUNServers,
		Relays:      []interface{}{},
	}, nil
}

func (s *Service) ServerVersion() map[string]any {
	return map[string]any{
		"server_version":               s.cfg.Version,
		"api_version":                  version.APIVersion,
		"min_supported_client_version": s.cfg.MinSupportedClientVersion,
		"latest_client_version":        s.cfg.LatestClientVersion,
	}
}

func (s *Service) deviceFromToken(ctx context.Context, bearer string) (sqlc.Device, error) {
	token := strings.TrimPrefix(strings.TrimSpace(bearer), "Bearer ")
	if token == "" {
		return sqlc.Device{}, ErrUnauthorized
	}
	device, err := s.store.Queries.GetDeviceByTokenHash(ctx, tokenHash(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Device{}, ErrUnauthorized
		}
		return sqlc.Device{}, err
	}
	if device.Status != "active" {
		return sqlc.Device{}, ErrUnauthorized
	}
	return device, nil
}

func nextUserCIDR(ctx context.Context, q *sqlc.Queries) (string, error) {
	lastCIDR, err := q.GetLastUserCIDR(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if errors.Is(err, pgx.ErrNoRows) || lastCIDR == "" {
		return "100.64.0.0/24", nil
	}
	prefix, err := netip.ParsePrefix(lastCIDR)
	if err != nil {
		return "", err
	}
	base, _ := netip.ParsePrefix(globalBaseCIDR)
	step := 1 << (32 - userCIDRPrefix)
	next := prefix.Addr()
	for i := 0; i < step; i++ {
		next = next.Next()
	}
	nextPrefix := netip.PrefixFrom(next, userCIDRPrefix)
	if !base.Contains(nextPrefix.Addr()) {
		return "", errors.New("overlay cidr pool exhausted")
	}
	return nextPrefix.String(), nil
}

func allocateDeviceIP(cidr string, devices []sqlc.Device) (string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", err
	}
	used := map[netip.Addr]bool{}
	for _, device := range devices {
		ip, err := netip.ParseAddr(device.VirtualIP)
		if err == nil {
			used[ip] = true
		}
	}
	addr := prefix.Addr()
	for prefix.Contains(addr) {
		if !isReservedDeviceIP(addr) && !used[addr] {
			return addr.String(), nil
		}
		addr = addr.Next()
	}
	return "", errors.New("no available virtual ip")
}

func isReservedDeviceIP(addr netip.Addr) bool {
	if !addr.Is4() {
		return false
	}
	octets := addr.As4()
	return octets[3] == 0 || octets[3] == 255
}

func effectiveDeviceLimit(limit int32) int32 {
	if limit <= 0 || limit > maxNodeLimit {
		return maxNodeLimit
	}
	return limit
}

func capabilitiesForPlan(code string) AccountCapabilities {
	switch code {
	case "relay":
		return AccountCapabilities{EnableSubnet: true, EnableSelfRelay: true}
	case "subnet":
		return AccountCapabilities{EnableSubnet: true, EnableSelfRelay: false}
	default:
		return AccountCapabilities{EnableSubnet: false, EnableSelfRelay: false}
	}
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(prefix string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") || strings.ContainsAny(email, " \t\r\n") {
		return "", errors.New("valid email is required")
	}
	return email, nil
}
