// Package config resolves Kite's provider settings from explicit flags, the
// environment, a user config file, and built-in defaults, in that order of
// precedence. The config file is additive and versioned (kite.config/v1) so
// future fields extend it without breaking existing installs.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Contract is the version identifier for the config file schema.
const Contract = "kite.config/v1"

// Built-in fallbacks used when no flag, environment variable, or config file
// value supplies a setting.
const (
	DefaultBaseURL = "https://api.openai.com/v1"
	DefaultModel   = "gpt-4o-mini"
)

// File is the persisted content of the user config file. Unknown fields are
// ignored so future versions stay readable by older binaries.
type File struct {
	// Version is the config contract, for example "kite.config/v1". It may
	// be omitted; an empty version is accepted.
	Version string `json:"version,omitempty"`
	// BaseURL is the OpenAI-compatible API base URL.
	BaseURL string `json:"base_url,omitempty"`
	// Model is the model identifier.
	Model string `json:"model,omitempty"`
	// APIKey is an optional credential. Prefer KeyEnv when the key should
	// stay out of the file.
	APIKey string `json:"api_key,omitempty"`
	// KeyEnv names an environment variable holding the API key. It takes
	// precedence over the inline APIKey when both are set.
	KeyEnv string `json:"key_env,omitempty"`
	// Theme is the default terminal theme name.
	Theme string `json:"theme,omitempty"`
	// DataDir overrides where sessions and artifacts are stored.
	DataDir string `json:"data_dir,omitempty"`
}

// Resolved is the effective provider configuration after applying the
// precedence flags > environment > config file > defaults.
type Resolved struct {
	BaseURL string
	Model   string
	APIKey  string
	// Source records where each value came from, for status output. Keys
	// are "base_url", "model", and "api_key"; values are one of "flag",
	// "env", "config", or "default".
	Source map[string]string
}

// Loaded pairs the parsed file with the path resolution was attempted at.
type Loaded struct {
	// Path is the config file location that was read or would be written.
	Path string
	// File is the parsed content, or zero-valued when no file existed.
	File File
	// Exists reports whether a config file was found on disk.
	Exists bool
}

// Load reads the config file from dir, or from the platform default location
// when dir is empty. A missing file is not an error; callers use Loaded.Exists
// to distinguish absent from empty. Malformed files return an error so users
// see broken configuration instead of silent defaults.
func Load(dir string) (*Loaded, error) {
	path := dir
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return &Loaded{}, nil
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Loaded{Path: path}, nil
		}
		return nil, fmt.Errorf("kite: read config: %w", err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("kite: parse config %s: %w", path, err)
	}
	if f.Version != "" && f.Version != Contract {
		return nil, fmt.Errorf("kite: unsupported config version %q in %s; expected %q", f.Version, path, Contract)
	}
	return &Loaded{Path: path, File: f, Exists: true}, nil
}

// Save writes cfg to path as user-only JSON, creating parent directories. An
// empty path selects the platform default location.
func Save(path string, cfg *File) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	v := *cfg
	if v.Version == "" {
		v.Version = Contract
	}
	data, err := json.MarshalIndent(&v, "", "  ")
	if err != nil {
		return fmt.Errorf("kite: encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("kite: create config dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("kite: write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("kite: replace config: %w", err)
	}
	return nil
}

// Resolve applies the precedence rules. Empty flag values fall through to the
// environment, then the config file, then the built-in defaults. The API key
// prefers KITE_API_KEY, then the file's KeyEnv-referenced variable, then the
// file's inline key.
func Resolve(flagBaseURL, flagModel string, loaded *Loaded) *Resolved {
	r := &Resolved{Source: map[string]string{}}
	var f File
	if loaded != nil {
		f = loaded.File
	}

	switch {
	case flagBaseURL != "":
		r.BaseURL, r.Source["base_url"] = flagBaseURL, "flag"
	case os.Getenv("KITE_BASE_URL") != "":
		r.BaseURL, r.Source["base_url"] = os.Getenv("KITE_BASE_URL"), "env"
	case f.BaseURL != "":
		r.BaseURL, r.Source["base_url"] = f.BaseURL, "config"
	default:
		r.BaseURL, r.Source["base_url"] = DefaultBaseURL, "default"
	}

	switch {
	case flagModel != "":
		r.Model, r.Source["model"] = flagModel, "flag"
	case os.Getenv("KITE_MODEL") != "":
		r.Model, r.Source["model"] = os.Getenv("KITE_MODEL"), "env"
	case f.Model != "":
		r.Model, r.Source["model"] = f.Model, "config"
	default:
		r.Model, r.Source["model"] = DefaultModel, "default"
	}

	switch {
	case os.Getenv("KITE_API_KEY") != "":
		r.APIKey, r.Source["api_key"] = os.Getenv("KITE_API_KEY"), "env"
	case f.KeyEnv != "" && os.Getenv(f.KeyEnv) != "":
		r.APIKey, r.Source["api_key"] = os.Getenv(f.KeyEnv), "config"
	case f.APIKey != "":
		r.APIKey, r.Source["api_key"] = f.APIKey, "config"
	default:
		r.Source["api_key"] = "unset"
	}

	return r
}
