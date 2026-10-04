package pipeline

import (
	"context"
	"encoding/json"
	"log/slog"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/llm"
)

type job struct {
	e               *Engine
	ctx             context.Context
	owner           string
	run             domain.Run
	refresh         bool
	use             domain.Usage
	docs            []domain.Document
	limitations     []string
	docCandidates   map[string]map[string]bool
	activeCandidate string
	attemptedURLs   map[string]bool
	requirements    domain.Requirements
	visionReads     int
	visualAttempts  map[string]bool
}

func (j *job) event(kind, message string) error {
	err := j.e.Store.Event(j.ctx, j.run.ID, kind, message)
	if err != nil {
		// Most trace events are best-effort; requirement transitions remain checked by callers.
		slog.Warn("cannot persist research trace", "run_id", j.run.ID, "event", kind)
	}
	return err
}

func (j *job) generate(instruction, input string, shape, out any) error {
	preamble, err := llm.Prompt("untrusted")
	if err != nil {
		return err
	}
	usage, err := llm.GenerateWithRetry(j.ctx, j.e.Model, preamble+"\n"+instruction, input, shape, out, j.modelCallLimit()-j.use.ModelCalls, func() error {
		return j.event("provider.retry", "Retrying one transient model failure within the existing call/time budget.")
	})
	j.use.ModelCalls += usage.ModelCalls
	j.use.InputTokens += usage.InputTokens
	j.use.OutputTokens += usage.OutputTokens
	j.use.UsageKnown = j.use.UsageKnown || usage.UsageKnown
	return err
}

func marshal(v any) string { b, _ := json.Marshal(v); return string(b) }

// modelCallLimit preserves the legacy default for explicit test configurations.
func (j *job) modelCallLimit() int {
	if j.e.Config.MaxModelCalls > 0 {
		return j.e.Config.MaxModelCalls
	}
	return 9
}

func (j *job) generatePrompt(name, input string, shape, out any) error {
	instruction, err := llm.Prompt(name)
	if err != nil {
		return err
	}
	return j.generate(instruction, input, shape, out)
}
