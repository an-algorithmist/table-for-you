package pipeline

import (
	"context"
	"strings"
	"testing"

	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
)

type noUsageModel struct{ calls int }

func (m *noUsageModel) Name() string { return "no-usage-fake" }
func (m *noUsageModel) Generate(context.Context, string, string, any, any) (domain.Usage, error) {
	m.calls++
	return domain.Usage{}, nil
}

// Provider errors can lack usage metadata; that must not bypass the run cap.
func TestRunBudgetCountsAttemptsWithoutUsageMetadata(t *testing.T) {
	model := &noUsageModel{}
	j := &job{ctx: context.Background(), e: &Engine{Model: model, Config: config.Config{MaxModelCalls: 2}}}
	for attempt := 0; attempt < 2; attempt++ {
		if err := j.generatePrompt("intent", "{}", domain.Interpretation{}, &domain.Interpretation{}); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}
	if err := j.generatePrompt("intent", "{}", domain.Interpretation{}, &domain.Interpretation{}); err == nil || !strings.Contains(err.Error(), "budget exhausted") {
		t.Fatalf("expected budget rejection, got %v", err)
	}
	if model.calls != 2 || j.use.ModelCalls != 2 {
		t.Fatalf("provider calls=%d, recorded calls=%d; want 2 each", model.calls, j.use.ModelCalls)
	}
}
