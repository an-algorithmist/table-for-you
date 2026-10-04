package pipeline

import (
	"errors"
	"fmt"
	"time"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/llm"
)

func (j *job) groundedResearch(req domain.Requirements, message string) (*domain.Result, error) {
	if !j.e.GroundedReady() {
		return nil, errors.New("google-grounded research is unavailable; use standard research")
	}
	_ = j.event("route.google_grounded", "Google-grounded research selected. One provider request; Google controls the number of searches. No automatic paid retry.")
	instruction, err := llm.Prompt("grounded")
	if err != nil {
		return nil, err
	}
	preamble, err := llm.Prompt("untrusted")
	if err != nil {
		return nil, err
	}
	instruction = preamble + instruction
	if j.use.ModelCalls >= j.modelCallLimit() {
		return nil, errors.New("model call budget exhausted")
	}
	ground, use, err := j.e.Grounded.Ground(j.ctx, instruction, marshal(map[string]any{"requirements": req, "new_message": message}), j.e.Config.GroundedMaxOutput)
	j.use.ModelCalls += max(1, use.ModelCalls)
	j.use.Searches += use.Searches
	j.use.Fetches += use.Fetches
	j.use.InputTokens += use.InputTokens
	j.use.OutputTokens += use.OutputTokens
	j.use.ThinkingTokens += use.ThinkingTokens
	j.use.ToolInputTokens += use.ToolInputTokens
	j.use.UsageKnown = j.use.UsageKnown && use.UsageKnown
	j.use.GroundedModel = use.GroundedModel
	if err != nil {
		return nil, err
	}
	for _, q := range ground.Queries {
		_ = j.event("grounded.search", q)
	}
	for _, u := range ground.URLs {
		_ = j.event("grounded.url", u.Status+": "+u.URL)
	}
	_ = j.event("grounded.completed", fmt.Sprintf("Returned %d source references and %d observed search queries. Grounding citations are provider attribution, not independent menu extraction.", len(ground.Sources), len(ground.Queries)))
	return &domain.Result{ValidationVersion: "google-grounded-v1", Requirements: req, Grounding: ground, Answer: ground.Text, Confirmed: []domain.Restaurant{}, Alternatives: []domain.Restaurant{}, Excluded: []domain.Restaurant{}, Sources: []domain.Document{}, ResearchedAt: time.Now().UTC(), Limitations: []string{"Google-grounded answer with provider source attribution; not independently validated by the standard menu extraction pipeline."}}, nil
}
