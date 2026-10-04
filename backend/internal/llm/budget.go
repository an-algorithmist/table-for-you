package llm

import (
	"context"
	"errors"
	"sync"
)

// ErrBudgetExhausted prevents another billable request after a run's call cap.
var ErrBudgetExhausted = errors.New("model call budget exhausted")

type budgetKey struct{}

// Budget counts attempted requests, including requests that fail before usage metadata arrives.
// It is shared by the plain, grounded and visual adapters within one research run.
type Budget struct {
	mu          sync.Mutex
	used, limit int
}

// WithBudget attaches a fresh run-scoped budget; it never changes the parent context.
func WithBudget(ctx context.Context, limit int) context.Context {
	if limit <= 0 {
		limit = 9
	}
	return context.WithValue(ctx, budgetKey{}, &Budget{limit: limit})
}

// ReserveCall consumes a slot before transport I/O. Standalone diagnostics have no run budget.
func ReserveCall(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, ok := ctx.Value(budgetKey{}).(*Budget)
	if !ok {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used >= b.limit {
		return ErrBudgetExhausted
	}
	b.used++
	return nil
}
