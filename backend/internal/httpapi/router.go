package httpapi

import (
	"net/http"
	"sync"
	"time"

	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/storage/postgres"
	web "table-for-you/frontend"
)

// ResearchEngine is the execution boundary consumed by HTTP handlers.
// Provider calls and orchestration remain outside the transport layer.
type ResearchEngine interface {
	Ready() bool
	GroundedReady() bool
	Start(ownerID string, run domain.Run, text string, refresh bool)
	Cancel(runID string)
}

// Server owns routing, browser sessions and HTTP request admission.
// Durable research state and SSE events remain in PostgreSQL so reconnects can replay them.
type Server struct {
	Store    *postgres.Store
	Engine   ResearchEngine
	Config   config.Config
	mu       sync.Mutex
	attempts map[string][]time.Time
}

// New constructs the application handler, including its same-origin boundaries.
func New(store *postgres.Store, engine ResearchEngine, cfg config.Config) http.Handler {
	server := &Server{Store: store, Engine: engine, Config: cfg, attempts: make(map[string][]time.Time)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /readyz", server.ready)
	mux.HandleFunc("GET /api/status", server.status)
	mux.HandleFunc("POST /api/access", server.access)
	mux.Handle("DELETE /api/access", server.protect(http.HandlerFunc(server.logout)))
	mux.Handle("POST /api/conversations", server.protect(http.HandlerFunc(server.create)))
	mux.Handle("GET /api/conversations", server.protect(http.HandlerFunc(server.list)))
	mux.Handle("GET /api/conversations/{id}", server.protect(http.HandlerFunc(server.conversation)))
	mux.Handle("DELETE /api/conversations/{id}", server.protect(http.HandlerFunc(server.delete)))
	mux.Handle("POST /api/conversations/{id}/messages", server.protect(http.HandlerFunc(server.message)))
	mux.Handle("POST /api/conversations/{id}/refresh", server.protect(http.HandlerFunc(server.message)))
	mux.Handle("GET /api/runs/{id}", server.protect(http.HandlerFunc(server.run)))
	mux.Handle("GET /api/runs/{id}/grounding-widget", server.protect(http.HandlerFunc(server.groundingWidget)))
	mux.Handle("GET /api/runs/{id}/events", server.protect(http.HandlerFunc(server.events)))
	mux.Handle("POST /api/runs/{id}/cancel", server.protect(http.HandlerFunc(server.cancel)))
	mux.Handle("GET /", http.FileServer(http.FS(web.Files())))
	return server.boundaries(mux)
}
