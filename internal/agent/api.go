package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type APIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

type RegisterRequest struct {
	JoinToken     string `json:"join_token"`
	Hostname      string `json:"hostname"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	OSVersion     string `json:"os_version"`
	PublicKey     string `json:"public_key"`
	ClientVersion string `json:"client_version"`
}

type RegisterResponse struct {
	DeviceID      string `json:"device_id"`
	DeviceToken   string `json:"device_token"`
	VirtualIP     string `json:"virtual_ip"`
	ControlURL    string `json:"control_url"`
	NetmapVersion int64  `json:"netmap_version"`
}

func (c *APIClient) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	var resp RegisterResponse
	if err := c.postJSON(ctx, "/api/v1/devices/register", "", req, &resp); err != nil {
		return RegisterResponse{}, err
	}
	return resp, nil
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
	ServerTime          time.Time `json:"server_time"`
	DeviceStatus        string    `json:"device_status"`
	NetmapVersion       int64     `json:"netmap_version"`
	NetmapChanged       bool      `json:"netmap_changed"`
	PollIntervalSeconds int       `json:"poll_interval_seconds"`
	Upgrade             struct {
		Required      bool   `json:"required"`
		Recommended   bool   `json:"recommended"`
		LatestVersion string `json:"latest_version"`
		Message       string `json:"message"`
	} `json:"upgrade"`
}

func (c *APIClient) Poll(ctx context.Context, token string, req PollRequest) (PollResponse, error) {
	var resp PollResponse
	if err := c.postJSON(ctx, "/api/v1/devices/poll", token, req, &resp); err != nil {
		return PollResponse{}, err
	}
	return resp, nil
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

func (c *APIClient) Netmap(ctx context.Context, token string) (Netmap, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/netmap", nil)
	if err != nil {
		return Netmap{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Netmap{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return Netmap{}, fmt.Errorf("controller returned %s", response.Status)
	}
	var netmap Netmap
	if err := json.NewDecoder(response.Body).Decode(&netmap); err != nil {
		return Netmap{}, err
	}
	return netmap, nil
}

func (c *APIClient) postJSON(ctx context.Context, path, token string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&payload)
		if payload.Error != "" {
			return fmt.Errorf("controller returned %s: %s", response.Status, payload.Error)
		}
		return fmt.Errorf("controller returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(output)
}
