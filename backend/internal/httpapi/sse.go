package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// events replays persisted events and then polls for new events until completion.
// Each stream owns its ticker and stops promptly when the client disconnects.
func (server *Server) events(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if _, e := server.Store.Run(r.Context(), owner(r), r.PathValue("id")); e != nil {
		server.problem(w, e)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, http.StatusInternalServerError, "Streaming unavailable.")
		return
	}
	after := eventCursor(r.Header.Get("Last-Event-ID"))
	if after == 0 {
		after = eventCursor(r.URL.Query().Get("after"))
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, ok := server.identity(r); !ok {
			return
		}
		events, e := server.Store.Events(r.Context(), r.PathValue("id"), after)
		if e != nil {
			return
		}
		for _, event := range events {
			payload, err := json.Marshal(event)
			if err != nil {
				slog.Error("encode SSE event", "run_id", r.PathValue("id"), "error", err)
				return
			}
			if _, err = fmt.Fprintf(w, "id: %d\nevent: progress\ndata: %s\n\n", event.Sequence, payload); err != nil {
				return
			}
			after = event.Sequence
		}
		flusher.Flush()
		run, e := server.Store.Run(r.Context(), owner(r), r.PathValue("id"))
		if e != nil {
			return
		}
		if run.Status != "running" {
			if _, err := fmt.Fprint(w, "event: done\ndata: {}\n\n"); err != nil {
				return
			}
			flusher.Flush()
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// An absent or invalid cursor means replay from the beginning, matching EventSource
// reconnection behavior. Query cursors are useful when restoring browser history.
func eventCursor(raw string) int64 {
	sequence, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return sequence
}
