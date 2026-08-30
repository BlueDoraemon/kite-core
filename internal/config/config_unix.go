//go:build !windows

package config

import (
	"os"
	"path/filepath"
)

// DefaultPath returns the XDG config file location for Kite on Unix systems.
func DefaultPath() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "kite", "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kite", "config.json"), nil
}
