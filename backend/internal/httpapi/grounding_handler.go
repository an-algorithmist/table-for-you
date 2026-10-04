package httpapi

import (
	"io"
	"log/slog"
	"net/http"
)

// Provider HTML is isolated from the app DOM, authenticated, and cannot run scripts.
func (server *Server) groundingWidget(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := server.Store.Run(r.Context(), owner(r), r.PathValue("id"))
	if err != nil {
		server.problem(w, err)
		return
	}
	if run.Result == nil || run.Result.Grounding == nil || run.Result.Grounding.SearchSuggestions == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src https: data:; script-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'; sandbox allow-popups allow-popups-to-escape-sandbox")
	if _, err := io.WriteString(w, run.Result.Grounding.SearchSuggestions); err != nil {
		slog.Warn("write isolated grounding widget", "error", err)
	}
}
