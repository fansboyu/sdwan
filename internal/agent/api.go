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
	baseURL string
	client  *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type RegisterRequest struct {
	AdminToken    string `json:"admin_token"`
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
}

type Netmap struct {
	Version     int64        `json:"version"`
	Self        NetmapSelf   `json:"self"`
	Peers       []NetmapPeer `json:"peers"`
	STUNServers []string     `json:"stun_servers"`
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

func (c *APIClient) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	var resp RegisterResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/devices/register", "", req, &resp)
	return resp, err
}

func (c *APIClient) Poll(ctx context.Context, token string, req PollRequest) (PollResponse, error) {
	var resp PollResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/devices/poll", token, req, &resp)
	return resp, err
}

func (c *APIClient) Netmap(ctx context.Context, token string) (Netmap, error) {
	var resp Netmap
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/netmap", token, nil, &resp)
	return resp, err
}

func (c *APIClient) doJSON(ctx context.Context, method, path, token string, body any, target any) error {
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errPayload struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errPayload)
		if errPayload.Error == "" {
			errPayload.Error = resp.Status
		}
		return fmt.Errorf("%s", errPayload.Error)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
