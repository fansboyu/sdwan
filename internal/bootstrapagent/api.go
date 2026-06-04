package bootstrapagent

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
	token   string
	client  *http.Client
}

func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type Peer struct {
	DeviceID   string   `json:"device_id"`
	Hostname   string   `json:"hostname"`
	PublicKey  string   `json:"public_key"`
	VirtualIP  string   `json:"virtual_ip"`
	Status     string   `json:"status"`
	AllowedIPs []string `json:"allowed_ips"`
}

type peersResponse struct {
	Peers []Peer `json:"peers"`
}

type endpointReport struct {
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
}

func (c *APIClient) Peers(ctx context.Context) ([]Peer, error) {
	var resp peersResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/bootstrap/peers", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Peers, nil
}

func (c *APIClient) ReportEndpoint(ctx context.Context, publicKey, endpoint string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/v1/bootstrap/endpoints", endpointReport{
		PublicKey: publicKey,
		Endpoint:  endpoint,
	}, nil)
}

func (c *APIClient) doJSON(ctx context.Context, method, path string, body any, target any) error {
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
