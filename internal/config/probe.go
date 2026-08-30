package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TestConnection sends one minimal, non-streaming chat completion to verify
// the base URL, credential, and model work together. It is used by setup so
// a bad key or endpoint is caught before anything is saved. Errors are
// secret-free: the credential never appears in the message.
func TestConnection(ctx context.Context, baseURL, apiKey, model string) error {
	if baseURL == "" {
		return fmt.Errorf("no base URL configured")
	}
	if model == "" {
		return fmt.Errorf("no model configured")
	}
	payload := map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with exactly: ok"}},
		"max_tokens": 8,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build probe: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %s", baseURL, withoutSecret(err.Error(), apiKey))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := strings.TrimSpace(string(detail))
		if msg == "" {
			msg = resp.Status
		} else {
			msg = truncateSafe(msg, 300)
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("authentication failed (%d): check the credential; the provider said: %s", resp.StatusCode, withoutSecret(msg, apiKey))
		case http.StatusPaymentRequired:
			return fmt.Errorf("the provider account has no credit (%d); top it up or choose another provider", resp.StatusCode)
		default:
			return fmt.Errorf("provider returned %d: %s", resp.StatusCode, withoutSecret(msg, apiKey))
		}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return fmt.Errorf("unexpected reply format from %s", baseURL)
	}
	if len(out.Choices) == 0 {
		return fmt.Errorf("endpoint replied with no choices; is %s an OpenAI-compatible chat completions root?", baseURL)
	}
	return nil
}

// withoutSecret removes the credential from an error string if a provider
// echoed it back.
func withoutSecret(s, secret string) string {
	if secret != "" && strings.Contains(s, secret) {
		return strings.ReplaceAll(s, secret, "[redacted]")
	}
	return s
}

func truncateSafe(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0x80 {
		cut = cut[:len(cut)-1]
	}
	return cut + "..."
}
