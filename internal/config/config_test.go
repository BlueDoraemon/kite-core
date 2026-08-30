package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	loaded, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Exists || loaded.File.Model != "" {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestLoadRejectsMalformedAndFutureVersions(t *testing.T) {
	dir := t.TempDir()
	bad := writeConfig(t, dir, `{not json`)
	if _, err := Load(bad); err == nil {
		t.Fatal("expected malformed config error")
	}
	future := writeConfig(t, dir, `{"version":"kite.config/v2","model":"m"}`)
	if _, err := Load(future); err == nil {
		t.Fatal("expected version error")
	} else if !contains(err.Error(), "kite.config/v2") {
		t.Fatalf("error should name the version: %v", err)
	}
}

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{"base_url":"https://cfg.example/v1","model":"cfg-model","api_key":"cfg-key"}`)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// Config file beats defaults.
	r := Resolve("", "", loaded)
	if r.BaseURL != "https://cfg.example/v1" || r.Model != "cfg-model" || r.APIKey != "cfg-key" {
		t.Fatalf("resolve from config = %+v", r)
	}
	if r.Source["base_url"] != "config" || r.Source["model"] != "config" || r.Source["api_key"] != "config" {
		t.Fatalf("sources = %+v", r.Source)
	}

	// Environment beats config.
	t.Setenv("KITE_MODEL", "env-model")
	t.Setenv("KITE_API_KEY", "env-key")
	r = Resolve("", "", loaded)
	if r.Model != "env-model" || r.APIKey != "env-key" {
		t.Fatalf("env should win: %+v", r)
	}

	// Flags beat environment.
	r = Resolve("https://flag.example/v1", "flag-model", loaded)
	if r.BaseURL != "https://flag.example/v1" || r.Model != "flag-model" {
		t.Fatalf("flags should win: %+v", r)
	}
	if r.Source["base_url"] != "flag" {
		t.Fatalf("sources = %+v", r.Source)
	}
}

func TestResolveKeyEnvReference(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{"key_env":"MY_KITE_KEY","api_key":"inline"}`)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Inline key is used when the referenced variable is unset.
	if r := Resolve("", "", loaded); r.APIKey != "inline" {
		t.Fatalf("unset env should fall back to inline: %+v", r)
	}
	t.Setenv("MY_KITE_KEY", "from-env-ref")
	if r := Resolve("", "", loaded); r.APIKey != "from-env-ref" {
		t.Fatalf("key_env should win over inline: %+v", r)
	}
}

func TestResolveDefaultsWhenNothingSet(t *testing.T) {
	r := Resolve("", "", &Loaded{})
	if r.BaseURL != DefaultBaseURL || r.Model != DefaultModel || r.APIKey != "" {
		t.Fatalf("defaults = %+v", r)
	}
	if r.Source["api_key"] != "unset" {
		t.Fatalf("sources = %+v", r.Source)
	}
}

func TestSaveRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg := &File{BaseURL: "https://a.example/v1", Model: "m", APIKey: "secret"}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config permissions = %o, want 600", perm)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Exists || loaded.File.Version != Contract || loaded.File.BaseURL != cfg.BaseURL || loaded.File.Model != cfg.Model || loaded.File.APIKey != cfg.APIKey {
		t.Fatalf("round trip = %+v", loaded)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
