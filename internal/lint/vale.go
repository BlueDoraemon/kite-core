package lint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type valeAlert struct {
	Span        []int  `json:"Span"`
	Check       string `json:"Check"`
	Message     string `json:"Message"`
	Severity    string `json:"Severity"`
	Line        int    `json:"Line"`
	Description string `json:"Description"`
}

func runVale(ctx context.Context, root string, paths []string, binary string) ([]Finding, error) {
	if binary == "" {
		binary = "vale"
	}
	args := []string{"--output=JSON"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, paths...)
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || len(out) == 0 {
			return nil, fmt.Errorf("lint: vale: %w", err)
		}
	}
	findings, parseErr := decodeVale(root, out)
	if parseErr != nil {
		return nil, fmt.Errorf("lint: vale output: %w", parseErr)
	}
	return findings, nil
}

func decodeVale(root string, data []byte) ([]Finding, error) {
	alerts := make(map[string][]valeAlert)
	if err := json.Unmarshal(data, &alerts); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(alerts))
	for path := range alerts {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var findings []Finding
	for _, rawPath := range paths {
		path := rawPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		path = filepath.Clean(path)
		if !within(root, path) {
			continue
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, alert := range alerts[rawPath] {
			column := 1
			if len(alert.Span) > 0 && alert.Span[0] > 0 {
				column = alert.Span[0]
			}
			severity := strings.ToLower(alert.Severity)
			if severity == "suggestion" {
				severity = "info"
			}
			if severity != "error" && severity != "warning" && severity != "info" {
				severity = "info"
			}
			rule := oneLine(alert.Check)
			if rule == "" {
				rule = "Vale"
			}
			message := oneLine(alert.Message)
			if message == "" {
				message = oneLine(alert.Description)
			}
			if alert.Line > 0 && message != "" {
				findings = append(findings, Finding{
					Layer: "vale", Rule: rule, Severity: severity, Path: rel,
					Line: alert.Line, Column: column, Message: message,
				})
			}
		}
	}
	return findings, nil
}
