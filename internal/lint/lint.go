// Package lint implements Kite's deterministic and model-assisted repository
// style checks. Deterministic findings are always produced without a provider.
package lint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// ContractVersion is the machine-output contract emitted by kite lint.
	ContractVersion = "kite.lint/v1"
	defaultMaxFile  = int64(1024 * 1024)
	defaultMaxInput = 128 * 1024
)

// Config controls a lint run.
type Config struct {
	Root        string
	Paths       []string
	MaxLine     int
	MaxFileSize int64
	MaxLLMInput int
	Vale        bool
	ValeBinary  string
	Reviewer    Reviewer
}

// Reviewer supplies the optional, advisory model-assisted layer.
type Reviewer interface {
	Review(context.Context, ReviewRequest) ([]Finding, error)
}

// ReviewRequest is the bounded, deterministic input sent to a Reviewer.
type ReviewRequest struct {
	Files []File `json:"files"`
}

// File is one text file selected for model review.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Finding is a single lint diagnostic.
type Finding struct {
	Layer      string `json:"layer"`
	Rule       string `json:"rule"`
	Severity   string `json:"severity"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Skipped describes a file omitted for a deterministic reason.
type Skipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Summary contains stable aggregate counts.
type Summary struct {
	Files         int `json:"files"`
	Deterministic int `json:"deterministic"`
	Vale          int `json:"vale"`
	LLM           int `json:"llm"`
	Errors        int `json:"errors"`
	Warnings      int `json:"warnings"`
	Info          int `json:"info"`
}

// Report is the versioned result of a layered lint run.
type Report struct {
	Version  string    `json:"version"`
	Layers   []string  `json:"layers"`
	Summary  Summary   `json:"summary"`
	Findings []Finding `json:"findings"`
	Skipped  []Skipped `json:"skipped,omitempty"`
}

// Run executes deterministic checks and then the optional review layer.
func Run(ctx context.Context, cfg Config) (*Report, error) {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, err
	}
	if cfg.MaxLine < 0 {
		return nil, fmt.Errorf("lint: max line must be non-negative")
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = defaultMaxFile
	}
	if cfg.MaxLLMInput <= 0 {
		cfg.MaxLLMInput = defaultMaxInput
	}

	paths, err := collect(root, cfg.Paths)
	if err != nil {
		return nil, err
	}
	report := &Report{Version: ContractVersion, Layers: []string{"deterministic"}}
	var reviewFiles []File
	remaining := cfg.MaxLLMInput
	for _, path := range paths {
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			report.Skipped = append(report.Skipped, Skipped{Path: rel, Reason: "not a regular file"})
			continue
		}
		if info.Size() > cfg.MaxFileSize {
			report.Skipped = append(report.Skipped, Skipped{Path: rel, Reason: "file exceeds size limit"})
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if isBinary(data) {
			report.Skipped = append(report.Skipped, Skipped{Path: rel, Reason: "binary file"})
			continue
		}
		report.Summary.Files++
		report.Findings = append(report.Findings, checkFile(rel, data, cfg.MaxLine)...)
		if cfg.Reviewer != nil && remaining > 0 && utf8.Valid(data) {
			n := len(data)
			if n > remaining {
				n = remaining
				for n > 0 && !utf8.Valid(data[:n]) {
					n--
				}
			}
			reviewFiles = append(reviewFiles, File{Path: rel, Content: string(data[:n])})
			remaining -= n
		}
	}
	if cfg.Vale {
		report.Layers = append(report.Layers, "vale")
		findings, err := runVale(ctx, root, cfg.Paths, cfg.ValeBinary)
		if err != nil {
			return nil, err
		}
		report.Findings = append(report.Findings, findings...)
	}

	if cfg.Reviewer != nil {
		report.Layers = append(report.Layers, "llm")
		findings, err := cfg.Reviewer.Review(ctx, ReviewRequest{Files: reviewFiles})
		if err != nil {
			return nil, fmt.Errorf("lint: llm review: %w", err)
		}
		allowed := make(map[string]int, len(reviewFiles))
		for _, file := range reviewFiles {
			allowed[file.Path] = 1 + strings.Count(file.Content, "\n")
		}
		for _, finding := range findings {
			if report.Summary.LLM >= 20 {
				break
			}
			if _, ok := allowed[finding.Path]; !ok {
				continue
			}
			if finding.Line < 1 || finding.Line > allowed[finding.Path] {
				continue
			}
			finding.Layer = "llm"
			finding.Rule = "LLM001"
			if finding.Severity != "warning" && finding.Severity != "info" {
				finding.Severity = "info"
			}
			if finding.Column < 1 {
				finding.Column = 1
			}
			finding.Message = oneLine(finding.Message)
			finding.Suggestion = oneLine(finding.Suggestion)
			if finding.Message != "" {
				report.Findings = append(report.Findings, finding)
				report.Summary.LLM++
			}
		}
	}

	sortFindings(report.Findings)
	sort.Slice(report.Skipped, func(i, j int) bool { return report.Skipped[i].Path < report.Skipped[j].Path })
	for _, finding := range report.Findings {
		switch finding.Layer {
		case "llm":
			// Counted while validating the bounded provider response.
		case "vale":
			report.Summary.Vale++
		default:
			report.Summary.Deterministic++
		}
		switch finding.Severity {
		case "error":
			report.Summary.Errors++
		case "warning":
			report.Summary.Warnings++
		default:
			report.Summary.Info++
		}
	}
	return report, nil
}

func checkFile(path string, data []byte, maxLine int) []Finding {
	if !utf8.Valid(data) {
		return []Finding{finding(path, 1, 1, "KITE004", "error", "file is not valid UTF-8")}
	}
	var out []Finding
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lines := bytes.Count(data, []byte{'\n'}) + 1
		out = append(out, finding(path, lines, 1, "KITE002", "warning", "file does not end with a newline"))
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimRight(line, " \t")
		if len(trimmed) != len(line) {
			out = append(out, finding(path, i+1, utf8.RuneCountInString(trimmed)+1, "KITE001", "warning", "trailing whitespace"))
		}
		if maxLine > 0 && utf8.RuneCountInString(line) > maxLine {
			out = append(out, finding(path, i+1, maxLine+1, "KITE005", "warning", fmt.Sprintf("line exceeds %d characters", maxLine)))
		}
		for col, r := range []rune(line) {
			if isUnsafeRune(r) {
				out = append(out, finding(path, i+1, col+1, "KITE006", "error", fmt.Sprintf("unsafe Unicode control U+%04X", r)))
			} else if r < 0x20 && r != '\t' {
				out = append(out, finding(path, i+1, col+1, "KITE007", "error", fmt.Sprintf("unexpected control character U+%04X", r)))
			}
		}
	}
	out = append(out, conflictFindings(path, lines)...)
	return out
}

func conflictFindings(path string, lines []string) []Finding {
	var out []Finding
	for start := 0; start < len(lines); start++ {
		if !strings.HasPrefix(lines[start], "<<<<<<< ") {
			continue
		}
		separator := -1
		for i := start + 1; i < len(lines); i++ {
			if lines[i] == "=======" {
				separator = i
				continue
			}
			if separator >= 0 && strings.HasPrefix(lines[i], ">>>>>>> ") {
				for _, line := range []int{start, separator, i} {
					out = append(out, finding(path, line+1, 1, "KITE003", "error", "unresolved merge conflict marker"))
				}
				start = i
				break
			}
		}
	}
	return out
}

func finding(path string, line, column int, rule, severity, message string) Finding {
	return Finding{Layer: "deterministic", Rule: rule, Severity: severity, Path: path, Line: line, Column: column, Message: message}
}

func collect(root string, requested []string) ([]string, error) {
	if len(requested) == 0 {
		cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
		cmd.Dir = root
		if out, err := cmd.Output(); err == nil {
			var paths []string
			for _, item := range bytes.Split(out, []byte{0}) {
				if len(item) > 0 {
					paths = append(paths, filepath.Join(root, filepath.FromSlash(string(item))))
				}
			}
			sort.Strings(paths)
			return paths, nil
		}
		requested = []string{"."}
	}

	seen := make(map[string]bool)
	var paths []string
	for _, raw := range requested {
		path := raw
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		path = filepath.Clean(path)
		if !within(root, path) {
			return nil, fmt.Errorf("lint: path %q escapes repository root", raw)
		}
		err := filepath.WalkDir(path, func(candidate string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			if !entry.IsDir() && !seen[candidate] {
				seen[candidate] = true
				paths = append(paths, candidate)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isBinary(data []byte) bool {
	limit := len(data)
	if limit > 8192 {
		limit = 8192
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}

func isUnsafeRune(r rune) bool {
	return r == 0x061c || r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Layer != b.Layer {
			return a.Layer < b.Layer
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Message < b.Message
	})
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

// DecodeReview parses the strict JSON object requested from a model.
func DecodeReview(data []byte) ([]Finding, error) {
	var response struct {
		Findings []Finding `json:"findings"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&response); err != nil {
		return nil, err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after review JSON")
	}
	return response.Findings, nil
}
