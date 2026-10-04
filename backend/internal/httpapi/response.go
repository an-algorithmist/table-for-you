package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"table-for-you/backend/internal/storage/postgres"
)

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode HTTP response", "status", status, "error", err)
	}
}

func fail(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}

func body(r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("JSON body required")
	}
	d := json.NewDecoder(io.LimitReader(r.Body, 16*1024))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return errors.New("invalid JSON body")
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return errors.New("one JSON object required")
	}
	return nil
}

func (server *Server) problem(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, postgres.ErrNotFound):
		fail(w, http.StatusNotFound, "Not found.")
	case errors.Is(e, postgres.ErrConflict):
		fail(w, http.StatusConflict, "Conversation changed or research is active. Reload and retry.")
	case errors.Is(e, postgres.ErrGroundedCapacity):
		fail(w, http.StatusTooManyRequests, "Google-grounded daily limit reached. Switch off Google-grounded research to use standard research.")
	case errors.Is(e, postgres.ErrCapacity):
		fail(w, http.StatusTooManyRequests, "Research capacity or daily quota reached. Try later.")
	default:
		slog.Error("database operation failed")
		fail(w, http.StatusServiceUnavailable, "Database unavailable. No new research was started.")
	}
}

func validID(w http.ResponseWriter, r *http.Request) bool {
	if _, e := uuid.Parse(r.PathValue("id")); e != nil {
		fail(w, http.StatusBadRequest, "Invalid identifier.")
		return false
	}
	return true
}
