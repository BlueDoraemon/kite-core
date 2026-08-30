package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupEnv returns a minimal environment with an isolated config home.
func setupEnv(t *testing.T) []string {
	t.Helper()
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"GOCACHE=" + os.Getenv("GOCACHE"),
		"XDG_CONFIG_HOME=" + t.TempDir(),
	}
}

func TestSetupNonInteractiveWritesPresetConfig(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "kite")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	env := setupEnv(t)
	cmd := exec.Command(bin, "setup", "-provider", "ollama", "-skip-test")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("setup: %v\n%s", err, out)
	}
	cfgPath := ""
	for line := range strings.SplitSeq(string(out), "\n") {
		if before, ok := strings.CutPrefix(line, "Saved "); ok {
			cfgPath = before
		}
	}
	if cfgPath == "" {
		t.Fatalf("no Saved line in:\n%s", out)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"kite.config/v1", "127.0.0.1:11434", "qwen3:8b"} {
		if !strings.Contains(text, want) {
			t.Fatalf("config missing %q:\n%s", want, text)
		}
	}
	info, _ := os.Stat(cfgPath)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config permissions = %o, want 600", perm)
	}
}

func TestSetupRejectsUnknownProvider(t *testing.T) {
	env := setupEnv(t)
	cmd := exec.Command("go", "run", ".", "setup", "-provider", "notreal", "-skip-test")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "unknown provider") {
		t.Fatalf("output = %s", out)
	}
}

func TestSetupProbeFailureBlocksSave(t *testing.T) {
	env := setupEnv(t)
	cmd := exec.Command("go", "run", ".", "setup", "-base-url", "http://127.0.0.1:9/v1", "-model", "m")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "connection test failed") {
		t.Fatalf("output = %s", out)
	}
}

func TestSetupInteractiveViaStdin(t *testing.T) {
	env := setupEnv(t)
	cmd := exec.Command("go", "run", ".", "setup", "-skip-test")
	cmd.Env = env
	cmd.Stdin = strings.NewReader("groq\n\n\nMY_GROQ_KEY\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("interactive setup: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "Providers:") || !strings.Contains(text, "Saved ") {
		t.Fatalf("output = %s", text)
	}
}
