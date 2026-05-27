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
	customerCIDRPrefix = 28
	globalBaseCIDR     = "100.64.0.0/10"
)

var ErrUnauthorized = errors.New("unauthorized")

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
	AdminUser sqlc.AdminUser `json:"admin_user"`
	Token     string         `json:"token"`
	ExpiresAt time.Time      `json:"expires_at"`
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
	user, err := s.store.Queries.CreateAdminUser(ctx, sqlc.CreateAdminUserParams{
		ID:           "adm_" + ksuid.New().String(),
		Email:        email,
		PasswordHash: string(passwordHash),
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
	user, err := s.store.Queries.GetAdminUserByEmail(ctx, email)
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

func (s *Service) AdminFromToken(ctx context.Context, bearer string) (sqlc.AdminUser, error) {
	token := strings.TrimPrefix(strings.TrimSpace(bearer), "Bearer ")
	if token == "" {
		return sqlc.AdminUser{}, ErrUnauthorized
	}
	user, err := s.store.Queries.GetAdminUserBySessionTokenHash(ctx, tokenHash(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.AdminUser{}, ErrUnauthorized
		}
		return sqlc.AdminUser{}, err
	}
	return user, nil
}

func (s *Service) createAdminSession(ctx context.Context, user sqlc.AdminUser) (AuthResponse, error) {
	token, err := randomToken("sdwan_admin")
	if err != nil {
		return AuthResponse{}, err
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	_, err = s.store.Queries.CreateAdminSession(ctx, sqlc.CreateAdminSessionParams{
		ID:          "ases_" + ksuid.New().String(),
		AdminUserID: user.ID,
		TokenHash:   tokenHash(token),
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return AuthResponse{}, err
	}
	user.PasswordHash = ""
	return AuthResponse{AdminUser: user, Token: token, ExpiresAt: expiresAt}, nil
}

type CreateCustomerRequest struct {
	Name string `json:"name"`
}

type CreateCustomerResponse struct {
	Customer  sqlc.Customer `json:"customer"`
	JoinToken string        `json:"join_token"`
}

func (s *Service) CreateCustomer(ctx context.Context, req CreateCustomerRequest) (CreateCustomerResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "default"
	}
	token, err := randomToken("sdwan_join")
	if err != nil {
		return CreateCustomerResponse{}, err
	}
	var customer sqlc.Customer
	err = s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.LockCustomerIPAllocator(ctx); err != nil {
			return err
		}
		cidr, err := nextCustomerCIDR(ctx, q)
		if err != nil {
			return err
		}
		customer, err = q.CreateCustomer(ctx, sqlc.CreateCustomerParams{
			ID:          "cus_" + ksuid.New().String(),
			Name:        name,
			AddressCidr: cidr,
			MaxDevices:  s.cfg.DefaultCustomerMaxDevices,
		})
		if err != nil {
			return err
		}
		_, err = q.CreateJoinToken(ctx, sqlc.CreateJoinTokenParams{
			ID:         "jtok_" + ksuid.New().String(),
			CustomerID: customer.ID,
			TokenHash:  tokenHash(token),
			Name:       "default",
			MaxUses:    customer.MaxDevices,
		})
		return err
	})
	return CreateCustomerResponse{Customer: customer, JoinToken: token}, nil
}

func (s *Service) ListCustomers(ctx context.Context) ([]sqlc.Customer, error) {
	return s.store.Queries.ListCustomers(ctx)
}

type RegisterDeviceRequest struct {
	JoinToken     string `json:"join_token"`
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
	joinToken, err := s.store.Queries.GetJoinTokenByHash(ctx, tokenHash(req.JoinToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RegisterDeviceResponse{}, ErrUnauthorized
		}
		return RegisterDeviceResponse{}, err
	}
	if joinToken.RevokedAt != nil || (joinToken.ExpiresAt != nil && joinToken.ExpiresAt.Before(time.Now())) {
		return RegisterDeviceResponse{}, ErrUnauthorized
	}
	if joinToken.UsedCount >= joinToken.MaxUses {
		return RegisterDeviceResponse{}, errors.New("join token usage limit reached")
	}

	customer, err := s.store.Queries.GetCustomer(ctx, joinToken.CustomerID)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	count, err := s.store.Queries.CountActiveDevicesByCustomer(ctx, customer.ID)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	if count >= customer.MaxDevices {
		return RegisterDeviceResponse{}, errors.New("customer device limit reached")
	}
	devices, err := s.store.Queries.ListDevicesByCustomer(ctx, customer.ID)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	virtualIP, err := allocateDeviceIP(customer.AddressCidr, devices)
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	deviceToken, err := randomToken("sdwan_device")
	if err != nil {
		return RegisterDeviceResponse{}, err
	}
	device, err := s.store.Queries.CreateDevice(ctx, sqlc.CreateDeviceParams{
		ID:              "dev_" + ksuid.New().String(),
		CustomerID:      customer.ID,
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
	if err := s.store.Queries.IncrementJoinTokenUse(ctx, joinToken.ID); err != nil {
		return RegisterDeviceResponse{}, err
	}
	if err := s.store.Queries.BumpNetmapVersion(ctx, customer.ID); err != nil {
		return RegisterDeviceResponse{}, err
	}
	customer, _ = s.store.Queries.GetCustomer(ctx, customer.ID)
	return RegisterDeviceResponse{
		DeviceID:      device.ID,
		DeviceToken:   deviceToken,
		VirtualIP:     device.VirtualIP,
		ControlURL:    s.cfg.ControllerURL,
		NetmapVersion: customer.NetmapVersion,
	}, nil
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
	for _, endpoint := range req.Endpoints {
		if strings.TrimSpace(endpoint.Address) == "" {
			continue
		}
		_ = s.store.Queries.UpsertDeviceEndpoint(ctx, sqlc.UpsertDeviceEndpointParams{
			ID:           "dep_" + ksuid.New().String(),
			DeviceID:     device.ID,
			EndpointType: defaultString(endpoint.Type, "unknown"),
			Address:      endpoint.Address,
			Source:       endpoint.Source,
			RttMs:        endpoint.RttMs,
		})
	}
	customer, err := s.store.Queries.GetCustomer(ctx, device.CustomerID)
	if err != nil {
		return PollResponse{}, err
	}
	return PollResponse{
		ServerTime:          time.Now().UTC(),
		DeviceStatus:        device.Status,
		NetmapVersion:       customer.NetmapVersion,
		NetmapChanged:       req.CurrentNetmapVersion != customer.NetmapVersion,
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
	customer, err := s.store.Queries.GetCustomer(ctx, self.CustomerID)
	if err != nil {
		return Netmap{}, err
	}
	devices, err := s.store.Queries.ListDevicesByCustomer(ctx, self.CustomerID)
	if err != nil {
		return Netmap{}, err
	}
	endpoints, err := s.store.Queries.ListEndpointsByCustomer(ctx, self.CustomerID)
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
		Version: customer.NetmapVersion,
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

func nextCustomerCIDR(ctx context.Context, q *sqlc.Queries) (string, error) {
	lastCIDR, err := q.GetLastCustomerCIDR(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if errors.Is(err, pgx.ErrNoRows) || lastCIDR == "" {
		return "100.64.0.0/28", nil
	}
	prefix, err := netip.ParsePrefix(lastCIDR)
	if err != nil {
		return "", err
	}
	next := prefix.Addr().Next()
	for i := 1; i < 16; i++ {
		next = next.Next()
	}
	return netip.PrefixFrom(next, customerCIDRPrefix).String(), nil
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
	for i := 0; i < 16; i++ {
		if prefix.Contains(addr) && !used[addr] {
			return addr.String(), nil
		}
		addr = addr.Next()
	}
	return "", errors.New("no available virtual ip")
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
