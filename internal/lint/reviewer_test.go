package lint

import (
	"context"
	"testing"

	"github.com/BlueDoraemon/kite-core/internal/core"
)

type reviewProvider struct {
	toolCount int
}

func (p *reviewProvider) Complete(_ context.Context, _ *core.Session, tools []core.Tool, onEvent func(core.ProviderEvent)) error {
	p.toolCount = len(tools)
	onEvent(core.ProviderEvent{Text: `{"findings":[{"path":"README.md","line":1,"column":1,"severity":"info","message":"Tighten the opening."}]}`})
	onEvent(core.ProviderEvent{Done: true})
	return nil
}

func TestSessionReviewerUsesToolFreeStructuredTurn(t *testing.T) {
	provider := &reviewProvider{}
	reviewer := SessionReviewer{Provider: provider, Model: "review-model"}
	findings, err := reviewer.Review(context.Background(), ReviewRequest{Files: []File{{Path: "README.md", Content: "A heading\n"}}})
	if err != nil {
		t.Fatal(err)
	}
	if provider.toolCount != 0 {
		t.Fatalf("review advertised %d tools, want 0", provider.toolCount)
	}
	if len(findings) != 1 || findings[0].Path != "README.md" {
		t.Fatalf("findings = %+v", findings)
	}
}
