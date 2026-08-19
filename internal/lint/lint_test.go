package lint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

type staticReviewer struct {
	request ReviewRequest
	result  []Finding
}

func (r *staticReviewer) Review(_ context.Context, request ReviewRequest) ([]Finding, error) {
	r.request = request
	return r.result, nil
}

func TestRunDeterministicFindingsAreStable(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "b.txt", "ok\n")
	mustWrite(t, root, "a.txt", "wide  \n<<<<<<< ours\na\n=======\nb\n>>>>>>> theirs")

	report, err := Run(context.Background(), Config{Root: root, Paths: []string{"."}, MaxLine: 120})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, finding := range report.Findings {
		got = append(got, finding.Path+":"+finding.Rule)
	}
	want := []string{"a.txt:KITE001", "a.txt:KITE003", "a.txt:KITE003", "a.txt:KITE002", "a.txt:KITE003"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	if report.Version != ContractVersion || report.Summary.Files != 2 || report.Summary.Deterministic != 5 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunBoundsAndValidatesLLMFindings(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "a.go", "package a\n")
	reviewer := &staticReviewer{result: []Finding{
		{Path: "a.go", Line: 1, Severity: "warning", Message: "  vague   name ", Suggestion: " rename it "},
		{Path: "invented.go", Line: 1, Severity: "error", Message: "hallucinated"},
		{Path: "a.go", Line: 99, Severity: "error", Message: "bad line"},
	}}
	report, err := Run(context.Background(), Config{Root: root, Paths: []string{"a.go"}, Reviewer: reviewer})
	if err != nil {
		t.Fatal(err)
	}
	if len(reviewer.request.Files) != 1 || reviewer.request.Files[0].Path != "a.go" {
		t.Fatalf("review request = %+v", reviewer.request)
	}
	if report.Summary.LLM != 1 || len(report.Findings) != 1 {
		t.Fatalf("report = %+v", report)
	}
	got := report.Findings[0]
	if got.Layer != "llm" || got.Rule != "LLM001" || got.Message != "vague name" || got.Suggestion != "rename it" {
		t.Fatalf("finding = %+v", got)
	}
}

func TestRunRejectsEscapingPath(t *testing.T) {
	root := t.TempDir()
	if _, err := Run(context.Background(), Config{Root: root, Paths: []string{"../outside"}}); err == nil {
		t.Fatal("Run accepted a path outside the repository")
	}
}

func TestDecodeReviewIsStrict(t *testing.T) {
	findings, err := DecodeReview([]byte(`{"findings":[{"path":"a.go","line":2,"severity":"info","message":"clear this up"}]}`))
	if err != nil || len(findings) != 1 {
		t.Fatalf("DecodeReview = %+v, %v", findings, err)
	}
	if _, err := DecodeReview([]byte("```json\n{\"findings\":[]}\n```")); err == nil {
		t.Fatal("DecodeReview accepted Markdown fences")
	}
	if _, err := DecodeReview([]byte(`{"findings":[],"extra":true}`)); err == nil {
		t.Fatal("DecodeReview accepted an unknown field")
	}
}

func TestDecodeValeNormalizesAlerts(t *testing.T) {
	root := t.TempDir()
	data := []byte(`{"docs/a.md":[{"Span":[7,10],"Check":"Kite.Terms","Message":"Prefer 'use'.","Severity":"suggestion","Line":3}]}`)
	findings, err := decodeVale(root, data)
	if err != nil {
		t.Fatal(err)
	}
	want := Finding{Layer: "vale", Rule: "Kite.Terms", Severity: "info", Path: "docs/a.md", Line: 3, Column: 7, Message: "Prefer 'use'."}
	if len(findings) != 1 || !reflect.DeepEqual(findings[0], want) {
		t.Fatalf("findings = %+v, want %+v", findings, want)
	}
}

func TestRunValeAcceptsAlertExitStatus(t *testing.T) {
	root := t.TempDir()
	helperDir := t.TempDir()
	helperSource := `package main
import (
    "fmt"
    "os"
)
func main() {
    fmt.Print("{\"docs/a.md\":[{\"Span\":[2,4],\"Check\":\"Demo.Term\",\"Message\":\"Prefer another term.\",\"Severity\":\"warning\",\"Line\":5}]}")
    os.Exit(1)
}`
	mustWrite(t, helperDir, "main.go", helperSource)
	helper := filepath.Join(helperDir, "vale-helper")
	if err := exec.Command("go", "build", "-o", helper, filepath.Join(helperDir, "main.go")).Run(); err != nil {
		t.Fatal(err)
	}
	findings, err := runVale(context.Background(), root, []string{"docs"}, helper)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != "Demo.Term" || findings[0].Severity != "warning" {
		t.Fatalf("findings = %+v", findings)
	}
}

func mustWrite(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
