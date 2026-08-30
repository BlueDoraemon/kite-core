package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPresetByName(t *testing.T) {
	if p := PresetByName("ollama"); p == nil || p.NeedsKey {
		t.Fatalf("ollama preset = %+v", p)
	}
	if p := PresetByName("openai"); p == nil || !p.NeedsKey || p.BaseURL == "" {
		t.Fatalf("openai preset = %+v", p)
	}
	if p := PresetByName("nope"); p != nil {
		t.Fatalf("unknown preset should be nil, got %+v", p)
	}
}

func TestTestConnectionSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer good-key" {
			t.Errorf("authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	defer srv.Close()
	if err := TestConnection(context.Background(), srv.URL, "good-key", "m"); err != nil {
		t.Fatalf("probe: %v", err)
	}
}

func TestTestConnectionAuthFailureIsSecretFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key sk-supersecret-abc"}}`))
	}))
	defer srv.Close()
	err := TestConnection(context.Background(), srv.URL, "sk-supersecret-abc", "m")
	if err == nil {
		t.Fatal("expected probe failure")
	}
	if strings.Contains(err.Error(), "sk-supersecret-abc") {
		t.Fatalf("error leaks the credential: %v", err)
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestTestConnectionUnreachable(t *testing.T) {
	err := TestConnection(context.Background(), "http://127.0.0.1:9/v1", "", "m")
	if err == nil || !strings.Contains(err.Error(), "cannot reach") {
		t.Fatalf("err = %v", err)
	}
}

func TestTestConnectionValidatesInputs(t *testing.T) {
	if err := TestConnection(context.Background(), "", "", "m"); err == nil {
		t.Fatal("expected base URL error")
	}
	if err := TestConnection(context.Background(), "http://x", "", ""); err == nil {
		t.Fatal("expected model error")
	}
}
