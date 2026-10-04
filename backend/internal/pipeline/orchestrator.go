package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/evidence"
	"table-for-you/backend/internal/llm"
	"table-for-you/backend/internal/providers"
)

func (e *Engine) execute(ctx context.Context, cancel context.CancelFunc, owner string, r domain.Run, text string, refresh bool) {
	// One context budget protects every real model adapter, including visual and grounded calls.
	ctx = llm.WithBudget(ctx, e.Config.MaxModelCalls)
	j := &job{use: domain.Usage{Mode: r.Usage.Mode}, e: e, ctx: ctx, owner: owner, run: r, refresh: refresh, docCandidates: map[string]map[string]bool{}}
	stopHeartbeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				c, err := e.Store.Heartbeat(ctx, r.ID)
				if err != nil || c {
					cancel()
					return
				}
			}
		}
	}()
	defer close(stopHeartbeat)
	var result *domain.Result
	var runErr error
	defer func() {
		if v := recover(); v != nil {
			runErr = errors.New("unexpected workflow failure")
			slog.Error("workflow panic", "run_id", r.ID)
		}
		status := "completed"
		message := ""
		if runErr != nil {
			status = "failed"
			message = providers.PublicError(runErr)
			if errors.Is(ctx.Err(), context.Canceled) {
				status = "cancelled"
			}
			if result != nil {
				status = "partial"
				result.Limitations = append(result.Limitations, message)
			}
		}
		if result != nil {
			result.Usage = j.use
		}
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer finishCancel()
		if err := e.Store.Finish(finishCtx, r, status, message, result, j.use); err != nil {
			slog.Error("cannot commit research result", "run_id", r.ID)
		}
		cleanupCtx, cc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cc()
		if err := e.Store.Cleanup(cleanupCtx); err != nil {
			slog.Warn("cannot clean expired research data", "run_id", r.ID)
		}
	}()
	if runErr = j.event("requirements.interpreting", "Interpreting your request and preserving existing mandatory restrictions."); runErr != nil {
		return
	}
	parsed, previous, err := j.interpret(text)
	if err != nil {
		runErr = err
		return
	}
	if !refresh && parsed.Action == "answer_existing" && previous != nil && marshal(previous.Requirements) == marshal(parsed.Requirements) && strings.TrimSpace(parsed.Answer) != "" {
		result, runErr = j.existingAnswer(parsed, previous)
		return
	}
	if parsed.Clarification != "" {
		result = &domain.Result{Clarification: parsed.Clarification, Requirements: parsed.Requirements, Confirmed: []domain.Restaurant{}, Alternatives: []domain.Restaurant{}, Excluded: []domain.Restaurant{}, Sources: []domain.Document{}}
		_ = j.event("clarification.required", parsed.Clarification)
		return
	}
	if runErr = j.event("requirements.updated", evidence.Summary(parsed.Requirements)); runErr != nil {
		return
	}
	j.requirements = parsed.Requirements
	if r.Usage.Mode == "google_grounded" {
		result, runErr = j.groundedResearch(parsed.Requirements, text)
		return
	}
	result, runErr = j.standardResearch(parsed.Requirements, text, parsed.Query)
}
