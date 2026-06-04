package relayagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"englishlisten/sdwan/internal/bootstrapagent"
	"englishlisten/sdwan/internal/storage/sqlc"
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

type peersResponse struct {
	Relay sqlc.Relay            `json:"relay"`
	Peers []bootstrapagent.Peer `json:"peers"`
}

func (c *APIClient) Peers(ctx context.Context) (peersResponse, error) {
	var resp peersResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/relays/peers", nil, &resp)
	return resp, err
}

func (c *APIClient) Heartbeat(ctx context.Context) error {
	return c.doJSON(ctx, http.MethodPost, "/api/v1/relays/heartbeat", map[string]string{}, nil)
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
