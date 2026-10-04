package pipeline

import (
	"context"
	"sync"

	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/storage/postgres"
)

// Engine coordinates asynchronous runs and owns their cancellation lifetimes.
type Engine struct {
	Grounded GroundedModel
	Store    *postgres.Store
	Model    Model
	Search   Search
	Config   config.Config
	root     context.Context
	mu       sync.Mutex
	running  map[string]context.CancelFunc
}

// New wires the run coordinator without starting background work.
func New(root context.Context, s *postgres.Store, m Model, q Search, c config.Config) *Engine {
	return &Engine{Store: s, Model: m, Search: q, Config: c, root: root, running: map[string]context.CancelFunc{}}
}

// GroundedReady reports whether the optional paid route is configured.
func (e *Engine) GroundedReady() bool {
	return e.Model != nil && e.Grounded != nil && e.Config.GroundedEnabled
}

// Ready reports whether the standard research dependencies are available.
func (e *Engine) Ready() bool { return e.Model != nil && e.Search != nil }

// Start owns one cancellable background run; completion is persisted for SSE readers.
func (e *Engine) Start(owner string, r domain.Run, text string, refresh bool) {
	ctx, cancel := context.WithTimeout(e.root, e.Config.RunTimeout)
	e.mu.Lock()
	e.running[r.ID] = cancel
	e.mu.Unlock()
	go func() {
		defer cancel()
		defer func() { e.mu.Lock(); delete(e.running, r.ID); e.mu.Unlock() }()
		e.execute(ctx, cancel, owner, r, text, refresh)
	}()
}

// Cancel stops an active run without deleting its stored history.
func (e *Engine) Cancel(id string) {
	e.mu.Lock()
	f := e.running[id]
	e.mu.Unlock()
	if f != nil {
		f()
	}
}
