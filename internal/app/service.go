package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"sort"
	"strings"
	"time"

	"englishlisten/sdwan/internal/config"
	"englishlisten/sdwan/internal/mailer"
	"englishlisten/sdwan/internal/storage"
	"englishlisten/sdwan/internal/storage/sqlc"
	"englishlisten/sdwan/internal/version"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/segmentio/ksuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	userCIDRPrefix                = 24
	globalBaseCIDR                = "100.64.0.0/10"
	defaultRelayVirtualIP         = "100.254.253.1"
	maxNodeLimit                  = 254
	ipAllocRetries                = 3
	endpointKeep                  = 3
	maxFreeUpgradeMonths          = 12
	subscriptionSourceFreeUpgrade = "free_upgrade"
	siteRoleClient                = "client"
	siteRoleMain                  = "main_site"
	emailPurposeRegister          = "register"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrUpgradeRequired = errors.New("upgrade required")
var ErrEmailAlreadyRegistered = errors.New("邮箱名已经注册，请更换其他邮箱")
var ErrEmailCodeNotConfigured = errors.New("邮件服务未配置，请联系管理员")
var ErrEmailCodeCooldown = errors.New("验证码发送太频繁，请稍后再试")
var ErrEmailCodeInvalid = errors.New("验证码错误，请重新输入")
var ErrEmailCodeExpired = errors.New("验证码已过期，请重新获取")
var ErrEmailCodeTooManyAttempts = errors.New("验证码尝试次数过多，请重新获取")

type Service struct {
	store *storage.Store
	cfg   config.Config
}

func NewService(store *storage.Store, cfg config.Config) *Service {
	return &Service{store: store, cfg: cfg}
}

type AuthRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	EmailCode string `json:"email_code"`
}

type EmailCodeRequest struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
}

type EmailCodeResponse struct {
	Sent             bool `json:"sent"`
	ExpiresInSeconds int  `json:"expires_in_seconds"`
	CooldownSeconds  int  `json:"cooldown_seconds"`
}

type AuthResponse struct {
	User      sqlc.User `json:"user"`
	AdminUser sqlc.User `json:"admin_user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) SendEmailCode(ctx context.Context, req EmailCodeRequest) (EmailCodeResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return EmailCodeResponse{}, err
	}
	purpose := defaultString(req.Purpose, emailPurposeRegister)
	if purpose != emailPurposeRegister {
		return EmailCodeResponse{}, errors.New("unsupported email code purpose")
	}
	if _, err := s.store.Queries.GetUserByEmail(ctx, email); err == nil {
		return EmailCodeResponse{}, ErrEmailAlreadyRegistered
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return EmailCodeResponse{}, err
	}
	if latest, err := s.store.Queries.GetLatestEmailVerification(ctx, email, purpose); err == nil {
		if time.Since(latest.CreatedAt) < s.cfg.EmailCodeCooldown {
			return EmailCodeResponse{}, ErrEmailCodeCooldown
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return EmailCodeResponse{}, err
	}
	code, err := randomNumericCode(6)
	if err != nil {
		return EmailCodeResponse{}, err
	}
	client := mailer.ResendClient{APIKey: s.cfg.ResendAPIKey, From: s.cfg.ResendFrom}
	if err := client.SendVerificationCode(ctx, email, code, s.cfg.EmailCodeTTL); err != nil {
		if errors.Is(err, mailer.ErrNotConfigured) {
			return EmailCodeResponse{}, ErrEmailCodeNotConfigured
		}
		return EmailCodeResponse{}, err
	}
	_, err = s.store.Queries.CreateEmailVerification(ctx, sqlc.CreateEmailVerificationParams{
		ID:        "evf_" + ksuid.New().String(),
		Email:     email,
		Purpose:   purpose,
		CodeHash:  emailCodeHash(email, purpose, code),
		ExpiresAt: time.Now().Add(s.cfg.EmailCodeTTL),
	})
	if err != nil {
		return EmailCodeResponse{}, err
	}
	return EmailCodeResponse{
		Sent:             true,
		ExpiresInSeconds: int(s.cfg.EmailCodeTTL.Seconds()),
		CooldownSeconds:  int(s.cfg.EmailCodeCooldown.Seconds()),
	}, nil
}

func (s *Service) RegisterAdmin(ctx context.Context, req AuthRequest) (AuthResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return AuthResponse{}, err
	}
	if len(req.Password) < 8 {
		return AuthResponse{}, errors.New("password must be at least 8 characters")
	}
	if _, err := s.store.Queries.GetUserByEmail(ctx, email); err == nil {
		return AuthResponse{}, ErrEmailAlreadyRegistered
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AuthResponse{}, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResponse{}, err
	}
	var user sqlc.User
	err = s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := verifyEmailCode(ctx, q, email, emailPurposeRegister, req.EmailCode, s.cfg.EmailCodeMaxAttempts); err != nil {
			return err
		}
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
		if isUserEmailConflict(err) {
			return AuthResponse{}, ErrEmailAlreadyRegistered
		}
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
	MainSite     *sqlc.Device        `json:"main_site,omitempty"`
	Plans        []sqlc.Plan         `json:"plans"`
	SubnetRoutes []sqlc.SubnetRoute  `json:"subnet_routes"`
	Relays       []sqlc.Relay        `json:"relays"`
	ActiveRelay  *sqlc.Relay         `json:"active_relay,omitempty"`
	Subscription *sqlc.Subscription  `json:"subscription,omitempty"`
	FreeUpgrade  FreeUpgradeSummary  `json:"free_upgrade"`
	Capabilities AccountCapabilities `json:"capabilities"`
	PeerPaths    []sqlc.PeerPath     `json:"peer_paths"`
	RelayHealthy bool                `json:"relay_healthy"`
}

type FreeUpgradeSummary struct {
	MonthsUsed      int32 `json:"months_used"`
	MonthsRemaining int32 `json:"months_remaining"`
	MaxMonths       int32 `json:"max_months"`
}

type AccountCapabilities struct {
	EnableSubnet    bool `json:"enable_subnet"`
	EnableSelfRelay bool `json:"enable_self_relay"`
}

func (s *Service) AccountSummary(ctx context.Context, user sqlc.User) (AccountSummary, error) {
	user, subscription, freeUsed, err := s.refreshSubscriptionState(ctx, user)
	if err != nil {
		return AccountSummary{}, err
	}
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
	paths, err := s.store.Queries.ListPeerPathsByUser(ctx, user.ID)
	if err != nil {
		return AccountSummary{}, err
	}
	devices, err := s.store.Queries.ListDevicesByUser(ctx, user.ID)
	if err != nil {
		return AccountSummary{}, err
	}
	var mainSite *sqlc.Device
	for _, device := range devices {
		if device.SiteRole == siteRoleMain && device.Status == "active" {
			copy := device
			mainSite = &copy
			break
		}
	}
	user.PasswordHash = ""
	active := activeRelay(relays)
	healthy := active != nil && relayIsHealthy(*active)
	return AccountSummary{
		User:         user,
		DeviceCount:  count,
		MainSite:     mainSite,
		Plans:        plans,
		SubnetRoutes: subnets,
		Relays:       relays,
		ActiveRelay:  active,
		Subscription: subscription,
		FreeUpgrade:  freeUpgradeSummary(freeUsed),
		Capabilities: capabilitiesForPlan(user.PlanCode),
		PeerPaths:    paths,
		RelayHealthy: healthy,
	}, nil
}

func (s *Service) ListPlans(ctx context.Context) ([]sqlc.Plan, error) {
	return s.store.Queries.ListPlans(ctx)
}

type CreateRelayRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
	VirtualIP string `json:"virtual_ip"`
}

type CreateRelayResponse struct {
	Relay      sqlc.Relay `json:"relay"`
	RelayToken string     `json:"relay_token"`
}

type RelayModeRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Service) CreateRelay(ctx context.Context, user sqlc.User, req CreateRelayRequest) (CreateRelayResponse, error) {
	if !capabilitiesForPlan(user.PlanCode).EnableSelfRelay {
		return CreateRelayResponse{}, ErrUpgradeRequired
	}
	publicKey := strings.TrimSpace(req.PublicKey)
	endpoint := strings.TrimSpace(req.Endpoint)
	if publicKey == "" || endpoint == "" {
		return CreateRelayResponse{}, errors.New("public_key and endpoint are required")
	}
	virtualIP := strings.TrimSpace(req.VirtualIP)
	if virtualIP == "" {
		virtualIP = defaultRelayVirtualIP
	}
	relayAddr, err := netip.ParseAddr(strings.TrimSuffix(virtualIP, "/32"))
	if err != nil {
		return CreateRelayResponse{}, errors.New("invalid relay virtual_ip")
	}
	virtualIP = relayAddr.String()
	relayToken, err := randomToken("sdwan_relay")
	if err != nil {
		return CreateRelayResponse{}, err
	}
	relay, err := s.store.Queries.CreateRelay(ctx, sqlc.CreateRelayParams{
		ID:             "rly_" + ksuid.New().String(),
		UserID:         user.ID,
		Name:           defaultString(req.Name, "relay"),
		PublicKey:      publicKey,
		RelayTokenHash: tokenHash(relayToken),
		VirtualIP:      virtualIP,
		Endpoint:       endpoint,
	})
	if err != nil {
		return CreateRelayResponse{}, err
	}
	return CreateRelayResponse{Relay: relay, RelayToken: relayToken}, nil
}

func (s *Service) EnableRelay(ctx context.Context, user sqlc.User, relayID string) (sqlc.Relay, error) {
	if !capabilitiesForPlan(user.PlanCode).EnableSelfRelay {
		return sqlc.Relay{}, ErrUpgradeRequired
	}
	var relay sqlc.Relay
	if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.DisableRelaysByUser(ctx, user.ID); err != nil {
			return err
		}
		updated, err := q.SetRelayStatus(ctx, relayID, user.ID, "active")
		if err != nil {
			return err
		}
		relay = updated
		return q.BumpNetmapVersion(ctx, user.ID)
	}); err != nil {
		return sqlc.Relay{}, err
	}
	return relay, nil
}

func (s *Service) DisableRelay(ctx context.Context, user sqlc.User, relayID string) (sqlc.Relay, error) {
	var relay sqlc.Relay
	if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		updated, err := q.SetRelayStatus(ctx, relayID, user.ID, "disabled")
		if err != nil {
			return err
		}
		relay = updated
		if err := q.UpdateUserRelayMode(ctx, user.ID, false); err != nil {
			return err
		}
		return q.BumpNetmapVersion(ctx, user.ID)
	}); err != nil {
		return sqlc.Relay{}, err
	}
	return relay, nil
}

func (s *Service) SetRelayMode(ctx context.Context, user sqlc.User, enabled bool) error {
	mode := pathModeDirect
	if enabled {
		mode = pathModeRelay
	}
	return s.SetPathMode(ctx, user, mode)
}

type SubscriptionSummary struct {
	Subscription *sqlc.Subscription `json:"subscription,omitempty"`
	FreeUpgrade  FreeUpgradeSummary `json:"free_upgrade"`
}

type FreeUpgradeRequest struct {
	PlanCode string `json:"plan_code"`
	Months   int32  `json:"months"`
}

func (s *Service) SubscriptionSummary(ctx context.Context, user sqlc.User) (SubscriptionSummary, error) {
	_, subscription, freeUsed, err := s.refreshSubscriptionState(ctx, user)
	if err != nil {
		return SubscriptionSummary{}, err
	}
	return SubscriptionSummary{
		Subscription: subscription,
		FreeUpgrade:  freeUpgradeSummary(freeUsed),
	}, nil
}

func (s *Service) FreeUpgrade(ctx context.Context, user sqlc.User, req FreeUpgradeRequest) (SubscriptionSummary, error) {
	planCode := strings.TrimSpace(req.PlanCode)
	if planCode != "subnet" && planCode != "relay" {
		return SubscriptionSummary{}, errors.New("plan_code must be subnet or relay")
	}
	if _, err := s.planByCode(ctx, planCode); err != nil {
		return SubscriptionSummary{}, err
	}
	_, activeSubscription, freeUsed, err := s.refreshSubscriptionState(ctx, user)
	if err != nil {
		return SubscriptionSummary{}, err
	}

	if activeSubscription != nil && activeSubscription.Source == subscriptionSourceFreeUpgrade {
		if planCode == activeSubscription.PlanCode {
			return SubscriptionSummary{
				Subscription: activeSubscription,
				FreeUpgrade:  freeUpgradeSummary(freeUsed),
			}, nil
		}
		if planRank(planCode) < planRank(activeSubscription.PlanCode) {
			return SubscriptionSummary{}, errors.New("当前版本已包含该能力")
		}
		var subscription sqlc.Subscription
		if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
			updated, err := q.UpdateActiveSubscriptionPlan(ctx, activeSubscription.ID, planCode)
			if err != nil {
				return err
			}
			subscription = updated
			return q.UpdateUserPlan(ctx, user.ID, planCode)
		}); err != nil {
			return SubscriptionSummary{}, err
		}
		return SubscriptionSummary{
			Subscription: &subscription,
			FreeUpgrade:  freeUpgradeSummary(freeUsed),
		}, nil
	}

	if req.Months <= 0 || req.Months > maxFreeUpgradeMonths {
		return SubscriptionSummary{}, errors.New("months must be between 1 and 12")
	}
	if freeUsed+req.Months > maxFreeUpgradeMonths {
		return SubscriptionSummary{}, errors.New("free upgrade months exceeded")
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, int(req.Months), 0)
	var subscription sqlc.Subscription
	if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.CancelActiveSubscriptionsByUser(ctx, user.ID); err != nil {
			return err
		}
		created, err := q.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
			ID:         "sub_" + ksuid.New().String(),
			UserID:     user.ID,
			PlanCode:   planCode,
			Source:     subscriptionSourceFreeUpgrade,
			FreeMonths: req.Months,
			StartsAt:   now,
			ExpiresAt:  &expiresAt,
		})
		if err != nil {
			return err
		}
		subscription = created
		return q.UpdateUserPlan(ctx, user.ID, planCode)
	}); err != nil {
		return SubscriptionSummary{}, err
	}

	freeUsed += req.Months
	return SubscriptionSummary{
		Subscription: &subscription,
		FreeUpgrade:  freeUpgradeSummary(freeUsed),
	}, nil
}

func (s *Service) CancelSubscription(ctx context.Context, user sqlc.User) (SubscriptionSummary, error) {
	if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.CancelActiveSubscriptionsByUser(ctx, user.ID); err != nil {
			return err
		}
		return q.UpdateUserPlan(ctx, user.ID, "free")
	}); err != nil {
		return SubscriptionSummary{}, err
	}
	return s.SubscriptionSummary(ctx, user)
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
	deviceToken, err := randomToken("sdwan_device")
	if err != nil {
		return RegisterDeviceResponse{}, err
	}

	var device sqlc.Device
	for attempt := 0; attempt < ipAllocRetries; attempt++ {
		devices, err := s.store.Queries.ListDevicesByUser(ctx, user.ID)
		if err != nil {
			return RegisterDeviceResponse{}, err
		}
		virtualIP, err := allocateDeviceIP(user.OverlayCidr, devices)
		if err != nil {
			return RegisterDeviceResponse{}, err
		}
		device, err = s.store.Queries.CreateDevice(ctx, sqlc.CreateDeviceParams{
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
		if err == nil {
			break
		}
		if !isDeviceVirtualIPConflict(err) {
			return RegisterDeviceResponse{}, err
		}
		if attempt == ipAllocRetries-1 {
			return RegisterDeviceResponse{}, errors.New("virtual ip allocation conflict; please retry")
		}
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
	if endpoints == nil {
		endpoints = []sqlc.DeviceEndpoint{}
	}
	user.PasswordHash = ""
	return DeviceDetail{Device: device, User: user, Endpoints: endpoints}, nil
}

func (s *Service) DeleteDevice(ctx context.Context, user sqlc.User, deviceID string) error {
	device, err := s.store.Queries.GetDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	if device.UserID != user.ID {
		return ErrUnauthorized
	}
	if err := s.store.Queries.DeleteDevice(ctx, device.ID); err != nil {
		return err
	}
	return s.store.Queries.BumpNetmapVersion(ctx, user.ID)
}

func (s *Service) SetMainSite(ctx context.Context, user sqlc.User, deviceID string) (sqlc.Device, error) {
	device, err := s.store.Queries.GetDevice(ctx, deviceID)
	if err != nil {
		return sqlc.Device{}, err
	}
	if device.UserID != user.ID {
		return sqlc.Device{}, ErrUnauthorized
	}
	if device.Status != "active" {
		return sqlc.Device{}, errors.New("device is not active")
	}
	if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.ClearMainSiteByUser(ctx, user.ID); err != nil {
			return err
		}
		if err := q.SetDeviceMainSite(ctx, device.ID, user.ID); err != nil {
			return err
		}
		if _, err := q.DisableSubnetRoutesExceptDevice(ctx, user.ID, device.ID); err != nil {
			return err
		}
		return q.BumpNetmapVersion(ctx, user.ID)
	}); err != nil {
		return sqlc.Device{}, err
	}
	device.SiteRole = siteRoleMain
	return device, nil
}

type SubnetRouteApprovalRequest struct {
	Approved bool `json:"approved"`
}

func (s *Service) ApproveSubnetRoute(ctx context.Context, user sqlc.User, routeID string, approved bool) (sqlc.SubnetRoute, error) {
	if !capabilitiesForPlan(user.PlanCode).EnableSubnet {
		return sqlc.SubnetRoute{}, ErrUpgradeRequired
	}
	var route sqlc.SubnetRoute
	if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		updated, err := q.SetSubnetRouteApproved(ctx, routeID, user.ID, approved)
		if err != nil {
			return err
		}
		route = updated
		return q.BumpNetmapVersion(ctx, user.ID)
	}); err != nil {
		return sqlc.SubnetRoute{}, err
	}
	return route, nil
}

func (s *Service) DisableSubnetRoute(ctx context.Context, user sqlc.User, routeID string) (sqlc.SubnetRoute, error) {
	var route sqlc.SubnetRoute
	if err := s.store.WithTx(ctx, func(q *sqlc.Queries) error {
		updated, err := q.DisableSubnetRoute(ctx, routeID, user.ID)
		if err != nil {
			return err
		}
		route = updated
		return q.BumpNetmapVersion(ctx, user.ID)
	}); err != nil {
		return sqlc.SubnetRoute{}, err
	}
	return route, nil
}

type EndpointReport struct {
	Type    string `json:"type"`
	Address string `json:"addr"`
	Source  string `json:"source"`
	RttMs   *int32 `json:"rtt_ms,omitempty"`
}

type PollRequest struct {
	CurrentNetmapVersion int64               `json:"current_netmap_version"`
	ClientVersion        string              `json:"client_version"`
	OSVersion            string              `json:"os_version"`
	Endpoints            []EndpointReport    `json:"endpoints"`
	AdvertiseRoutes      []string            `json:"advertise_routes,omitempty"`
	PeerStats            []PeerStatReport    `json:"peer_stats,omitempty"`
	AppliedPaths         []AppliedPathReport `json:"applied_paths,omitempty"`
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
	if err := s.recordPathReports(ctx, device, req.PeerStats, req.AppliedPaths); err != nil {
		return PollResponse{}, err
	}
	endpointChanged := false
	for _, endpoint := range req.Endpoints {
		if strings.TrimSpace(endpoint.Address) == "" {
			continue
		}
		if endpoint.Type == "stun" {
			continue
		}
		endpointType := defaultString(endpoint.Type, "unknown")
		changed, err := s.store.Queries.UpsertDeviceEndpoint(ctx, sqlc.UpsertDeviceEndpointParams{
			ID:           "dep_" + ksuid.New().String(),
			DeviceID:     device.ID,
			EndpointType: endpointType,
			Address:      endpoint.Address,
			Source:       endpoint.Source,
			RttMs:        endpoint.RttMs,
		})
		if err != nil {
			return PollResponse{}, err
		}
		deleted, err := s.store.Queries.PruneDeviceEndpoints(ctx, sqlc.PruneDeviceEndpointsParams{
			DeviceID:     device.ID,
			EndpointType: endpointType,
			KeepCount:    endpointKeep,
		})
		if err != nil {
			return PollResponse{}, err
		}
		endpointChanged = endpointChanged || changed || deleted > 0
	}
	if endpointChanged {
		if err := s.store.Queries.BumpNetmapVersion(ctx, device.UserID); err != nil {
			return PollResponse{}, err
		}
	}
	if req.AdvertiseRoutes != nil {
		changed, err := s.syncAdvertisedSubnetRoutes(ctx, device, req.AdvertiseRoutes)
		if err != nil {
			return PollResponse{}, err
		}
		if changed {
			if err := s.store.Queries.BumpNetmapVersion(ctx, device.UserID); err != nil {
				return PollResponse{}, err
			}
		}
	}
	user, err := s.store.Queries.GetUser(ctx, device.UserID)
	if err != nil {
		return PollResponse{}, err
	}
	if changed, err := s.reconcilePeerPaths(ctx, user); err != nil {
		return PollResponse{}, err
	} else if changed {
		if err := s.store.Queries.BumpNetmapVersion(ctx, user.ID); err != nil {
			return PollResponse{}, err
		}
		user, err = s.store.Queries.GetUser(ctx, device.UserID)
		if err != nil {
			return PollResponse{}, err
		}
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

func (s *Service) syncAdvertisedSubnetRoutes(ctx context.Context, device sqlc.Device, routes []string) (bool, error) {
	normalized, err := normalizeSubnetRoutes(routes)
	if err != nil {
		return false, err
	}
	if len(normalized) > 0 {
		if device.SiteRole != siteRoleMain {
			return false, errors.New("only main site can advertise subnet routes")
		}
		user, err := s.store.Queries.GetUser(ctx, device.UserID)
		if err != nil {
			return false, err
		}
		if !capabilitiesForPlan(user.PlanCode).EnableSubnet {
			return false, ErrUpgradeRequired
		}
	}
	changed := false
	for _, cidr := range normalized {
		routeChanged, err := s.store.Queries.UpsertAdvertisedSubnetRoute(ctx, sqlc.UpsertAdvertisedSubnetRouteParams{
			ID:       "srt_" + ksuid.New().String(),
			UserID:   device.UserID,
			DeviceID: device.ID,
			Cidr:     cidr,
		})
		if err != nil {
			return false, err
		}
		changed = changed || routeChanged
	}
	deleted, err := s.store.Queries.DisableDeviceSubnetRoutesNotIn(ctx, device.ID, normalized)
	if err != nil {
		return false, err
	}
	return changed || deleted > 0, nil
}

type Netmap struct {
	Version         int64            `json:"version"`
	OverlayCIDR     string           `json:"overlay_cidr"`
	Self            NetmapSelf       `json:"self"`
	Peers           []NetmapPeer     `json:"peers"`
	BootstrapPeer   *NetmapPeer      `json:"bootstrap_peer,omitempty"`
	Relays          []interface{}    `json:"relays"`
	PathMode        string           `json:"path_mode"`
	PathGeneration  int64            `json:"path_generation"`
	PathAssignments []PathAssignment `json:"path_assignments,omitempty"`
}

type NetmapSelf struct {
	DeviceID  string `json:"device_id"`
	Hostname  string `json:"hostname"`
	VirtualIP string `json:"virtual_ip"`
	PublicKey string `json:"public_key"`
	SiteRole  string `json:"site_role"`
}

type NetmapPeer struct {
	DeviceID            string   `json:"device_id"`
	Hostname            string   `json:"hostname"`
	VirtualIP           string   `json:"virtual_ip"`
	PublicKey           string   `json:"public_key"`
	AllowedIPs          []string `json:"allowed_ips"`
	Endpoints           []string `json:"endpoints"`
	PersistentKeepalive int      `json:"persistent_keepalive"`
	PathRole            string   `json:"path_role,omitempty"`
	PathActive          bool     `json:"path_active,omitempty"`
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
	activeRoutes, err := s.store.Queries.ListActiveSubnetRoutesByUser(ctx, self.UserID)
	if err != nil {
		return Netmap{}, err
	}
	endpointsByDevice := map[string][]sqlc.DeviceEndpoint{}
	for _, endpoint := range endpoints {
		endpointsByDevice[endpoint.DeviceID] = append(endpointsByDevice[endpoint.DeviceID], endpoint)
	}
	routesByDevice := map[string][]string{}
	for _, route := range activeRoutes {
		routesByDevice[route.DeviceID] = append(routesByDevice[route.DeviceID], route.Cidr)
	}
	if netmap, ok, err := s.pathAwareNetmap(ctx, user, self, devices, endpointsByDevice, routesByDevice, activeRoutes); err != nil {
		return Netmap{}, err
	} else if ok {
		netmap.BootstrapPeer = s.bootstrapPeer()
		return netmap, nil
	}

	if user.RelayMode && capabilitiesForPlan(user.PlanCode).EnableSelfRelay {
		if relay, err := s.store.Queries.GetActiveRelayByUser(ctx, self.UserID); err == nil {
			allowedIPs := []string{user.OverlayCidr}
			if self.SiteRole != siteRoleMain {
				for _, route := range activeRoutes {
					allowedIPs = append(allowedIPs, route.Cidr)
				}
			}
			return Netmap{
				Version:     user.NetmapVersion,
				OverlayCIDR: user.OverlayCidr,
				Self: NetmapSelf{
					DeviceID:  self.ID,
					Hostname:  self.Hostname,
					VirtualIP: self.VirtualIP,
					PublicKey: self.PublicKey,
					SiteRole:  defaultString(self.SiteRole, siteRoleClient),
				},
				Peers: []NetmapPeer{{
					DeviceID:            relay.ID,
					Hostname:            relay.Name,
					VirtualIP:           relay.VirtualIP,
					PublicKey:           relay.PublicKey,
					AllowedIPs:          dedupeStrings(allowedIPs),
					Endpoints:           []string{relay.Endpoint},
					PersistentKeepalive: 25,
				}},
				BootstrapPeer: s.bootstrapPeer(),
				Relays:        []interface{}{relay},
			}, nil
		}
	}

	visibleDevices := hubAndSpokePeers(self, devices)
	peers := make([]NetmapPeer, 0, len(visibleDevices))
	for _, d := range visibleDevices {
		peerEndpoints := orderedEndpointAddresses(endpointsByDevice[d.ID])
		allowedIPs := []string{d.VirtualIP + "/32"}
		if d.SiteRole == siteRoleMain && self.SiteRole != siteRoleMain {
			allowedIPs = append(allowedIPs, routesByDevice[d.ID]...)
		}
		peers = append(peers, NetmapPeer{
			DeviceID:            d.ID,
			Hostname:            d.Hostname,
			VirtualIP:           d.VirtualIP,
			PublicKey:           d.PublicKey,
			AllowedIPs:          allowedIPs,
			Endpoints:           peerEndpoints,
			PersistentKeepalive: 25,
		})
	}
	return Netmap{
		Version:     user.NetmapVersion,
		OverlayCIDR: user.OverlayCidr,
		Self: NetmapSelf{
			DeviceID:  self.ID,
			Hostname:  self.Hostname,
			VirtualIP: self.VirtualIP,
			PublicKey: self.PublicKey,
			SiteRole:  defaultString(self.SiteRole, siteRoleClient),
		},
		Peers:         peers,
		BootstrapPeer: s.bootstrapPeer(),
		Relays:        []interface{}{},
	}, nil
}

func hubAndSpokePeers(self sqlc.Device, devices []sqlc.Device) []sqlc.Device {
	var mainSite *sqlc.Device
	clients := make([]sqlc.Device, 0, len(devices))
	for _, device := range devices {
		if device.ID == self.ID || device.Status != "active" {
			continue
		}
		if device.SiteRole == siteRoleMain {
			copy := device
			mainSite = &copy
			continue
		}
		clients = append(clients, device)
	}
	if self.SiteRole == siteRoleMain {
		return clients
	}
	if mainSite == nil {
		return nil
	}
	return []sqlc.Device{*mainSite}
}

func orderedEndpointAddresses(endpoints []sqlc.DeviceEndpoint) []string {
	if len(endpoints) == 0 {
		return nil
	}
	items := append([]sqlc.DeviceEndpoint(nil), endpoints...)
	sort.SliceStable(items, func(i, j int) bool {
		left := endpointPriority(items[i].EndpointType)
		right := endpointPriority(items[j].EndpointType)
		if left != right {
			return left < right
		}
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		return items[i].Address < items[j].Address
	})
	result := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item.Address) == "" || seen[item.Address] {
			continue
		}
		seen[item.Address] = true
		result = append(result, item.Address)
	}
	return result
}

func endpointPriority(endpointType string) int {
	switch endpointType {
	case "bootstrap":
		return 0
	case "manual":
		return 1
	case "lan":
		return 2
	case "ipv6":
		return 3
	default:
		return 9
	}
}

type BootstrapEndpointReportRequest struct {
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
}

type BootstrapPeer struct {
	DeviceID  string `json:"device_id"`
	Hostname  string `json:"hostname"`
	PublicKey string `json:"public_key"`
	VirtualIP string `json:"virtual_ip"`
	Status    string `json:"status"`
}

type BootstrapPeersResponse struct {
	Peers []BootstrapPeer `json:"peers"`
}

type RelayPeer struct {
	DeviceID   string   `json:"device_id"`
	Hostname   string   `json:"hostname"`
	PublicKey  string   `json:"public_key"`
	VirtualIP  string   `json:"virtual_ip"`
	Status     string   `json:"status"`
	AllowedIPs []string `json:"allowed_ips"`
}

type RelayPeersResponse struct {
	Relay sqlc.Relay  `json:"relay"`
	Peers []RelayPeer `json:"peers"`
}

func (s *Service) RelayPeers(ctx context.Context, bearer string) (RelayPeersResponse, error) {
	relay, err := s.relayFromToken(ctx, bearer)
	if err != nil {
		return RelayPeersResponse{}, err
	}
	if relay.Status != "active" {
		return RelayPeersResponse{}, ErrUnauthorized
	}
	devices, err := s.store.Queries.ListDevicesByUser(ctx, relay.UserID)
	if err != nil {
		return RelayPeersResponse{}, err
	}
	activeRoutes, err := s.store.Queries.ListActiveSubnetRoutesByUser(ctx, relay.UserID)
	if err != nil {
		return RelayPeersResponse{}, err
	}
	routesByDevice := map[string][]string{}
	for _, route := range activeRoutes {
		routesByDevice[route.DeviceID] = append(routesByDevice[route.DeviceID], route.Cidr)
	}
	peers := make([]RelayPeer, 0, len(devices))
	for _, device := range devices {
		if device.Status != "active" || strings.TrimSpace(device.PublicKey) == "" {
			continue
		}
		allowedIPs := []string{device.VirtualIP + "/32"}
		if device.SiteRole == siteRoleMain {
			allowedIPs = append(allowedIPs, routesByDevice[device.ID]...)
		}
		peers = append(peers, RelayPeer{
			DeviceID:   device.ID,
			Hostname:   device.Hostname,
			PublicKey:  device.PublicKey,
			VirtualIP:  device.VirtualIP,
			Status:     device.Status,
			AllowedIPs: allowedIPs,
		})
	}
	return RelayPeersResponse{Relay: relay, Peers: peers}, nil
}

func (s *Service) RelayHeartbeat(ctx context.Context, bearer string) error {
	relay, err := s.relayFromToken(ctx, bearer)
	if err != nil {
		return err
	}
	return s.store.Queries.UpdateRelayHeartbeat(ctx, relay.ID)
}

func (s *Service) BootstrapPeers(ctx context.Context, bearer string) (BootstrapPeersResponse, error) {
	if err := s.authorizeBootstrap(bearer); err != nil {
		return BootstrapPeersResponse{}, err
	}
	devices, err := s.store.Queries.ListActiveDevices(ctx)
	if err != nil {
		return BootstrapPeersResponse{}, err
	}
	peers := make([]BootstrapPeer, 0, len(devices))
	for _, device := range devices {
		peers = append(peers, BootstrapPeer{
			DeviceID:  device.ID,
			Hostname:  device.Hostname,
			PublicKey: device.PublicKey,
			VirtualIP: device.VirtualIP,
			Status:    device.Status,
		})
	}
	return BootstrapPeersResponse{Peers: peers}, nil
}

func (s *Service) ReportBootstrapEndpoint(ctx context.Context, bearer string, req BootstrapEndpointReportRequest) error {
	if err := s.authorizeBootstrap(bearer); err != nil {
		return err
	}
	publicKey := strings.TrimSpace(req.PublicKey)
	endpoint := strings.TrimSpace(req.Endpoint)
	if publicKey == "" || endpoint == "" {
		return errors.New("public_key and endpoint are required")
	}
	device, err := s.store.Queries.GetDeviceByPublicKey(ctx, publicKey)
	if err != nil {
		return err
	}
	changed, err := s.store.Queries.UpsertDeviceEndpoint(ctx, sqlc.UpsertDeviceEndpointParams{
		ID:           "dep_" + ksuid.New().String(),
		DeviceID:     device.ID,
		EndpointType: "bootstrap",
		Address:      endpoint,
		Source:       "wg-bootstrap",
	})
	if err != nil {
		return err
	}
	deleted, err := s.store.Queries.PruneDeviceEndpoints(ctx, sqlc.PruneDeviceEndpointsParams{
		DeviceID:     device.ID,
		EndpointType: "bootstrap",
		KeepCount:    endpointKeep,
	})
	if err != nil {
		return err
	}
	if changed || deleted > 0 {
		return s.store.Queries.BumpNetmapVersion(ctx, device.UserID)
	}
	return nil
}

func (s *Service) authorizeBootstrap(bearer string) error {
	if s.cfg.BootstrapReportToken == "" || strings.TrimPrefix(strings.TrimSpace(bearer), "Bearer ") != s.cfg.BootstrapReportToken {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) bootstrapPeer() *NetmapPeer {
	if strings.TrimSpace(s.cfg.BootstrapPublicKey) == "" || strings.TrimSpace(s.cfg.BootstrapEndpoint) == "" {
		return nil
	}
	allowedIP := strings.TrimSpace(s.cfg.BootstrapAllowedIP)
	if allowedIP == "" {
		allowedIP = "100.254.254.254/32"
	}
	return &NetmapPeer{
		DeviceID:            "bootstrap",
		Hostname:            "controller-bootstrap",
		VirtualIP:           strings.TrimSuffix(allowedIP, "/32"),
		PublicKey:           strings.TrimSpace(s.cfg.BootstrapPublicKey),
		AllowedIPs:          []string{allowedIP},
		Endpoints:           []string{strings.TrimSpace(s.cfg.BootstrapEndpoint)},
		PersistentKeepalive: 25,
	}
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

func (s *Service) relayFromToken(ctx context.Context, bearer string) (sqlc.Relay, error) {
	token := strings.TrimPrefix(strings.TrimSpace(bearer), "Bearer ")
	if token == "" {
		return sqlc.Relay{}, ErrUnauthorized
	}
	relay, err := s.store.Queries.GetRelayByTokenHash(ctx, tokenHash(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Relay{}, ErrUnauthorized
		}
		return sqlc.Relay{}, err
	}
	return relay, nil
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

func normalizeSubnetRoutes(routes []string) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0, len(routes))
	overlay, _ := netip.ParsePrefix(globalBaseCIDR)
	for _, route := range routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(route)
		if err != nil {
			return nil, errors.New("invalid subnet route cidr: " + route)
		}
		prefix = prefix.Masked()
		if !prefix.Addr().Is4() {
			return nil, errors.New("only IPv4 subnet routes are supported")
		}
		if prefix.Bits() == 0 {
			return nil, errors.New("default route 0.0.0.0/0 is not supported")
		}
		if prefixesOverlap(prefix, overlay) {
			return nil, errors.New("subnet route cannot overlap overlay cidr " + globalBaseCIDR)
		}
		normalized := prefix.String()
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result, nil
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func activeRelay(relays []sqlc.Relay) *sqlc.Relay {
	for _, relay := range relays {
		if relay.Status == "active" {
			copy := relay
			return &copy
		}
	}
	return nil
}

func prefixesOverlap(left, right netip.Prefix) bool {
	return left.Contains(right.Addr()) || right.Contains(left.Addr())
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

func freeUpgradeSummary(used int32) FreeUpgradeSummary {
	remaining := maxFreeUpgradeMonths - used
	if remaining < 0 {
		remaining = 0
	}
	return FreeUpgradeSummary{
		MonthsUsed:      used,
		MonthsRemaining: remaining,
		MaxMonths:       maxFreeUpgradeMonths,
	}
}

func planRank(code string) int {
	switch code {
	case "relay":
		return 2
	case "subnet":
		return 1
	default:
		return 0
	}
}

func (s *Service) refreshSubscriptionState(ctx context.Context, user sqlc.User) (sqlc.User, *sqlc.Subscription, int32, error) {
	expired, err := s.store.Queries.ExpireActiveSubscriptionsByUser(ctx, user.ID)
	if err != nil {
		return sqlc.User{}, nil, 0, err
	}
	if expired > 0 {
		if err := s.store.Queries.UpdateUserPlan(ctx, user.ID, "free"); err != nil {
			return sqlc.User{}, nil, 0, err
		}
	}

	updatedUser, err := s.store.Queries.GetUser(ctx, user.ID)
	if err != nil {
		return sqlc.User{}, nil, 0, err
	}
	updatedUser.PasswordHash = ""

	freeUsed, err := s.store.Queries.SumFreeUpgradeMonthsByUser(ctx, user.ID)
	if err != nil {
		return sqlc.User{}, nil, 0, err
	}

	subscription, err := s.store.Queries.GetActiveSubscriptionByUser(ctx, user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return updatedUser, nil, freeUsed, nil
		}
		return sqlc.User{}, nil, 0, err
	}
	if subscription.ExpiresAt != nil && !subscription.ExpiresAt.After(time.Now().UTC()) {
		return updatedUser, nil, freeUsed, nil
	}
	return updatedUser, &subscription, freeUsed, nil
}

func (s *Service) planByCode(ctx context.Context, code string) (sqlc.Plan, error) {
	plans, err := s.store.Queries.ListPlans(ctx)
	if err != nil {
		return sqlc.Plan{}, err
	}
	for _, plan := range plans {
		if plan.Code == code {
			return plan, nil
		}
	}
	return sqlc.Plan{}, errors.New("plan not found")
}

func isDeviceVirtualIPConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "devices_user_id_virtual_ip_key"
}

func isUserEmailConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key"
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

func emailCodeHash(email, purpose, code string) string {
	sum := sha256.Sum256([]byte(email + "|" + purpose + "|" + code))
	return hex.EncodeToString(sum[:])
}

func verifyEmailCode(ctx context.Context, q *sqlc.Queries, email, purpose, code string, maxAttempts int32) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrEmailCodeInvalid
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	verification, err := q.GetLatestEmailVerification(ctx, email, purpose)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEmailCodeInvalid
		}
		return err
	}
	if verification.ConsumedAt != nil {
		return ErrEmailCodeInvalid
	}
	if time.Now().After(verification.ExpiresAt) {
		return ErrEmailCodeExpired
	}
	if verification.AttemptCount >= maxAttempts {
		return ErrEmailCodeTooManyAttempts
	}
	if verification.CodeHash != emailCodeHash(email, purpose, code) {
		_ = q.IncrementEmailVerificationAttempts(ctx, verification.ID)
		return ErrEmailCodeInvalid
	}
	return q.ConsumeEmailVerification(ctx, verification.ID)
}

func randomNumericCode(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	max := big.NewInt(10)
	var b strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(&b, "%d", n.Int64())
	}
	return b.String(), nil
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
