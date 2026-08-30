//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// DefaultPath returns the AppData config file location for Kite on Windows.
func DefaultPath() (string, error) {
	if v := os.Getenv("APPDATA"); v != "" {
		return filepath.Join(v, "kite", "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "AppData", "Roaming", "kite", "config.json"), nil
}
