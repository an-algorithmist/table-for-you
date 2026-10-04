package llm

import (
	"context"
	"errors"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/providers"
	"time"
)

// Generator is the structured-output boundary consumed by the retry policy.
type Generator interface {
	Generate(context.Context, string, string, any, any) (domain.Usage, error)
}

// GenerateWithRetry retries one transient plain-model failure within remaining calls.
// Grounding never uses this policy: retrying it could repeat expensive provider searches.
func GenerateWithRetry(ctx context.Context, model Generator, instruction, input string, shape, out any, remaining int, onRetry func() error) (domain.Usage, error) {
	var total domain.Usage
	for attempt := 0; attempt < 2; attempt++ {
		if total.ModelCalls >= remaining {
			return total, ErrBudgetExhausted
		}
		u, err := model.Generate(ctx, instruction, input, shape, out)
		total.ModelCalls += max(1, u.ModelCalls)
		total.InputTokens += u.InputTokens
		total.OutputTokens += u.OutputTokens
		total.UsageKnown = total.UsageKnown || u.UsageKnown
		var failure *providers.Error
		if err == nil || attempt == 1 || !errors.As(err, &failure) || !transient(failure.Status) {
			return total, err
		}
		if onRetry != nil {
			if err := onRetry(); err != nil {
				return total, err
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return total, ctx.Err()
		case <-timer.C:
		}
	}
	return total, errors.New("model call failed")
}

func transient(status int) bool {
	return status == 500 || status == 502 || status == 503 || status == 504
}
