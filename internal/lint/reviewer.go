package lint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/BlueDoraemon/kite-core/internal/core"
)

// SessionReviewer adapts a Kite provider into the read-only lint review
// layer. It advertises no tools and stores its short-lived session in a
// private temporary directory.
type SessionReviewer struct {
	Provider core.Provider
	Model    string
}

// Review asks the configured provider for strict, structured advisory
// findings. The deterministic layer has already run before this method.
func (r SessionReviewer) Review(ctx context.Context, request ReviewRequest) ([]Finding, error) {
	if r.Provider == nil || r.Model == "" {
		return nil, fmt.Errorf("reviewer is not configured")
	}
	if len(request.Files) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	dataDir, err := os.MkdirTemp("", "kite-lint-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dataDir)

	session, err := core.NewSession(core.Config{
		Provider: r.Provider,
		Model:    r.Model,
		DataDir:  dataDir,
		Tools:    []core.Tool{},
		MaxTurns: 1,
	})
	if err != nil {
		return nil, err
	}
	prompt := `Act as Kite's advisory repository style reviewer. The deterministic lint layer has already checked whitespace, final newlines, merge markers, UTF-8, line length, and unsafe controls.

Review only the supplied source. Report high-confidence clarity, consistency, documentation, naming, or maintainability issues. Do not report correctness or security claims that the source does not establish. Return exactly one JSON object with this shape and no Markdown:
{"findings":[{"path":"exact supplied path","line":1,"column":1,"severity":"warning or info","message":"concise issue","suggestion":"concise action"}]}

Use only supplied paths and valid line numbers. Return at most 20 findings. Return {"findings":[]} when there are none.

SOURCE:
` + string(payload)
	events, err := session.Prompt(ctx, prompt)
	if err != nil {
		return nil, err
	}
	var result *core.Result
	var failure *core.Error
	for event := range events {
		switch event.Type {
		case core.EventSessionCompleted:
			result = event.Payload.(*core.SessionCompletedPayload).Result
		case core.EventSessionFailed:
			failure = event.Payload.(*core.SessionFailedPayload).Error
		}
	}
	if failure != nil {
		return nil, failure
	}
	if result == nil {
		return nil, fmt.Errorf("review ended without a result")
	}
	return DecodeReview([]byte(result.Text))
}
