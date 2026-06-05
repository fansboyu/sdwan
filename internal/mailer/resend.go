package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("email service is not configured")

type ResendClient struct {
	APIKey string
	From   string
	HTTP   *http.Client
}

func (c ResendClient) SendVerificationCode(ctx context.Context, email, code string, ttl time.Duration) error {
	if strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(c.From) == "" {
		return ErrNotConfigured
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	minutes := int(ttl.Minutes())
	if minutes <= 0 {
		minutes = 10
	}
	payload := map[string]any{
		"from":    c.From,
		"to":      []string{email},
		"subject": "你的 SD-WAN 控制台验证码",
		"text": strings.Join([]string{
			"你的 SD-WAN 控制台验证码是：",
			"",
			code,
			"",
			fmt.Sprintf("验证码 %d 分钟内有效。", minutes),
			"如果不是你本人操作，请忽略这封邮件。",
		}, "\n"),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("resend email failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
}
