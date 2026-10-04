package llm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBudgetBoundsConcurrentAttempts(t *testing.T) {
	ctx := WithBudget(context.Background(), 3)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if ReserveCall(ctx) == nil {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 3 {
		t.Fatalf("accepted %d attempts, want 3", accepted.Load())
	}
	if !errors.Is(ReserveCall(ctx), ErrBudgetExhausted) {
		t.Fatal("budget was reset")
	}
	if err := ReserveCall(WithBudget(context.Background(), 1)); err != nil {
		t.Fatal("independent run inherited exhausted budget")
	}
}
