package httpapi

import (
	"log/slog"
	"net/http"
)

func (server *Server) run(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	server.recoverRuns(r)
	v, e := server.Store.Run(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		server.problem(w, e)
		return
	}
	reply(w, http.StatusOK, v)
}

func (server *Server) cancel(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	e := server.Store.Cancel(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		server.problem(w, e)
		return
	}
	server.Engine.Cancel(r.PathValue("id"))
	reply(w, http.StatusOK, map[string]bool{"ok": true})
}

// Recovery is best-effort here: reading already persisted state should still succeed
// if another worker temporarily holds the maintenance lock.
func (server *Server) recoverRuns(r *http.Request) {
	if err := server.Store.Recover(r.Context()); err != nil {
		slog.Warn("recover expired research leases", "error", err)
	}
}
