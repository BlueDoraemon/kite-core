package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BlueDoraemon/kite-core/internal/core"
	"github.com/BlueDoraemon/kite-core/internal/provider/openai"
	"github.com/BlueDoraemon/kite-core/internal/rpc"
)

func TestRPCPromptReturnsRuntimeFailure(t *testing.T) {
	t.Setenv("KITE_DATA_DIR", filepath.Join(t.TempDir(), "data"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()
	handler := &rpcHandler{provider: openai.New(server.URL, "", "m"), dir: t.TempDir()}
	params, _ := json.Marshal(rpc.PromptParams{Text: "hello"})
	resp, err := handler.Handle(&rpc.Request{ID: "1", Method: rpc.MethodPrompt, Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK || resp.Error == nil || resp.Error.Code != "provider" {
		t.Fatalf("RPC response = %+v", resp)
	}
}

func TestRPCStatusReturnsRequestedSession(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	t.Setenv("KITE_DATA_DIR", dataDir)
	workDir := t.TempDir()
	session, err := core.NewSession(core.Config{Provider: core.NoopProvider{}, Model: "m", WorkingDir: workDir, DataDir: dataDir, Tools: []core.Tool{}})
	if err != nil {
		t.Fatal(err)
	}
	events, err := session.Prompt(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}
	handler := &rpcHandler{provider: openai.New("http://unused", "", "m"), dir: workDir}
	params, _ := json.Marshal(rpc.StatusParams{SessionID: session.ID})
	resp, err := handler.Handle(&rpc.Request{ID: "1", Method: rpc.MethodStatus, Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("RPC response = %+v", resp)
	}
	var status rpc.StatusResult
	if err := json.Unmarshal(resp.Result, &status); err != nil {
		t.Fatal(err)
	}
	if status.SessionID != session.ID || status.Messages == 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestTUIRejectsUnknownThemeBeforeSessionSetup(t *testing.T) {
	if code := run([]string{"tui", "-theme", "ultraviolet"}); code != 2 {
		t.Fatalf("run(tui invalid theme) = %d, want 2", code)
	}
}

// TestRunGuidesWhenNoCredentialConfigured checks the first-run path: with no
// key in the environment or config file, a remote default endpoint produces
// actionable guidance rather than a provider 401.
func TestRunGuidesWhenNoCredentialConfigured(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "run", "hello")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"GOCACHE=" + os.Getenv("GOCACHE"),
		"XDG_CONFIG_HOME=" + t.TempDir(),
		"KITE_DATA_DIR=" + filepath.Join(t.TempDir(), "data"),
	}
	out, err := cmd.CombinedOutput()
	text := string(out)
	if !strings.Contains(text, "no model credential configured") || !strings.Contains(text, "KITE_API_KEY") {
		t.Fatalf("guidance missing from output:\n%s", text)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("error = %v, want failure", err)
	}
}

// TestRunReadsBaseURLFromConfigFile proves the config file is consulted by
// the CLI: a request goes to the configured (unreachable) local endpoint.
func TestRunReadsBaseURLFromConfigFile(t *testing.T) {
	cfgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfgDir, "kite"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := `{"version":"kite.config/v1","base_url":"http://127.0.0.1:9/v1","model":"m","api_key":"k"}`
	if err := os.WriteFile(filepath.Join(cfgDir, "kite", "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "run", "hello")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"GOCACHE=" + os.Getenv("GOCACHE"),
		"XDG_CONFIG_HOME=" + cfgDir,
		"KITE_DATA_DIR=" + filepath.Join(t.TempDir(), "data"),
	}
	out, err := cmd.CombinedOutput()
	text := string(out)
	if !strings.Contains(text, "127.0.0.1:9") {
		t.Fatalf("expected request to the configured endpoint:\n%s", text)
	}
	if err == nil {
		t.Fatal("expected connection failure")
	}
}

func TestTUIExecutableRunsPlainInteractiveSession(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "tui", "-plain", "-theme", "paper-trail")
	cmd.Env = append(os.Environ(), "KITE_DATA_DIR="+filepath.Join(t.TempDir(), "data"), "KITE_API_KEY=test-key")
	cmd.Stdin = strings.NewReader("/theme high-contrast\n/quit\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kite tui failed: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{"KITE | session sess_", "theme paper-trail", "theme set to high-contrast", "session retained as sess_"} {
		if !strings.Contains(text, want) {
			t.Fatalf("kite tui output missing %q:\n%s", want, text)
		}
	}
	if strings.ContainsRune(text, '\x1b') {
		t.Fatalf("kite tui -plain emitted ANSI escapes: %q", text)
	}
}

func TestLintExecutableEmitsVersionedJSONAndFindingExit(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "kite")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build kite: %v\n%s", err, out)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.txt"), []byte("trailing  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "lint", "-json", "bad.txt")
	cmd.Dir = root
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("kite lint error = %v, output = %s", err, out)
	}
	var report struct {
		Version  string `json:"version"`
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("decode lint JSON: %v\n%s", err, out)
	}
	if report.Version != "kite.lint/v1" || len(report.Findings) != 1 || report.Findings[0].Rule != "KITE001" {
		t.Fatalf("report = %+v", report)
	}
}
