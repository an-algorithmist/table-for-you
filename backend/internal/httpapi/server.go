package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"nebulaiq/internal/config"
	"nebulaiq/internal/storage/postgres"
	"nebulaiq/internal/workflow"
	web "table-for-you/frontend"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Identity struct{ OwnerID string }
type contextKey struct{}
type Server struct {
	Store    *postgres.Store
	Engine   *workflow.Engine
	Config   config.Config
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func New(s *postgres.Store, e *workflow.Engine, c config.Config) http.Handler {
	a := &Server{s, e, c, sync.Mutex{}, map[string][]time.Time{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", a.ready)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		_, auth := a.identity(r)
		reply(w, 200, map[string]any{"authenticated": auth, "local": c.Local, "providers_ready": e.Ready(), "model": c.Model, "auth_stage": "guest", "history_retention_days": 7, "google_grounded_available": e.GroundedReady(), "grounded_owner_limit": c.GroundedOwnerQuota, "grounded_global_limit": c.GroundedGlobalQuota})
	})
	mux.HandleFunc("POST /api/access", a.access)
	mux.Handle("DELETE /api/access", a.protect(http.HandlerFunc(a.logout)))
	mux.Handle("POST /api/conversations", a.protect(http.HandlerFunc(a.create)))
	mux.Handle("GET /api/conversations", a.protect(http.HandlerFunc(a.list)))
	mux.Handle("GET /api/conversations/{id}", a.protect(http.HandlerFunc(a.conversation)))
	mux.Handle("DELETE /api/conversations/{id}", a.protect(http.HandlerFunc(a.delete)))
	mux.Handle("POST /api/conversations/{id}/messages", a.protect(http.HandlerFunc(a.message)))
	mux.Handle("POST /api/conversations/{id}/refresh", a.protect(http.HandlerFunc(a.message)))
	mux.Handle("GET /api/runs/{id}", a.protect(http.HandlerFunc(a.run)))
	mux.Handle("GET /api/runs/{id}/grounding-widget", a.protect(http.HandlerFunc(a.groundingWidget)))
	mux.Handle("GET /api/runs/{id}/events", a.protect(http.HandlerFunc(a.events)))
	mux.Handle("POST /api/runs/{id}/cancel", a.protect(http.HandlerFunc(a.cancel)))
	mux.Handle("GET /", http.FileServer(http.FS(web.Files())))
	return a.boundaries(mux)
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
func (a *Server) boundaries(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != "GET" && r.Method != "HEAD" {
				if r.Header.Get("X-Nebula-Request") != "1" {
					fail(w, 403, "Same-origin request header required.")
					return
				}
				if raw := r.Header.Get("Origin"); raw != "" {
					u, e := url.Parse(raw)
					if e != nil || u.Host != r.Host || (u.Scheme != "https" && u.Scheme != "http") {
						fail(w, 403, "Cross-origin requests are not allowed.")
						return
					}
				}
			}
		}
		defer func() {
			if recover() != nil {
				slog.Error("HTTP handler panic", "path", r.URL.Path)
				fail(w, 500, "Request failed.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (a *Server) identity(r *http.Request) (Identity, bool) {
	cookie, e := r.Cookie("nebula_session")
	if e != nil {
		return Identity{}, false
	}
	owner, e := a.Store.Owner(r.Context(), cookie.Value)
	return Identity{owner}, e == nil
}
func (a *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, cookieError := r.Cookie("nebula_session")
		if cookieError != nil {
			fail(w, 401, "Your browser session expired. Open the dining notebook again.")
			return
		}
		ownerID, err := a.Store.Owner(r.Context(), cookie.Value)
		if errors.Is(err, postgres.ErrNotFound) {
			fail(w, 401, "Your browser session expired. Open the dining notebook again.")
			return
		}
		if err != nil {
			a.problem(w, err)
			return
		}
		v := Identity{OwnerID: ownerID}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, v)))
	})
}
func owner(r *http.Request) string { return r.Context().Value(contextKey{}).(Identity).OwnerID }
func (a *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, c := context.WithTimeout(r.Context(), 3*time.Second)
	defer c()
	if e := a.Store.Pool.Ping(ctx); e != nil {
		fail(w, 503, "Database unavailable.")
		return
	}
	reply(w, 200, map[string]string{"status": "ready"})
}
func (a *Server) allowAttempt(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	recent := []time.Time{}
	for _, t := range a.attempts[host] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		return false
	}
	recent = append(recent, now)
	if len(a.attempts) > 1000 {
		a.attempts = map[string][]time.Time{}
	}
	a.attempts[host] = recent
	return true
}
func (a *Server) access(w http.ResponseWriter, r *http.Request) {
	if !a.allowAttempt(r) {
		fail(w, 429, "Too many access attempts; wait one minute.")
		return
	}
	var v struct {
		Code string `json:"code"`
	}
	if e := body(r, &v); e != nil {
		fail(w, 400, e.Error())
		return
	}
	expected, provided := postgres.Hash(a.Config.AccessCode), postgres.Hash(v.Code)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		fail(w, 401, "Access code not accepted.")
		return
	}
	if _, ok := a.identity(r); ok {
		reply(w, 200, map[string]bool{"ok": true})
		return
	}
	token, e := a.Store.NewSession(r.Context())
	if e != nil {
		a.problem(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "nebula_session", Value: token, Path: "/", HttpOnly: true, Secure: !a.Config.Local, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 3600})
	reply(w, 200, map[string]bool{"ok": true})
}
func (a *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("nebula_session")
	if e := a.Store.Revoke(r.Context(), cookie.Value); e != nil {
		a.problem(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "nebula_session", Value: "", Path: "/", HttpOnly: true, Secure: !a.Config.Local, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	reply(w, 200, map[string]bool{"ok": true})
}
func (a *Server) problem(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, postgres.ErrNotFound):
		fail(w, 404, "Not found.")
	case errors.Is(e, postgres.ErrConflict):
		fail(w, 409, "Conversation changed or research is active. Reload and retry.")
	case errors.Is(e, postgres.ErrGroundedCapacity):
		fail(w, 429, "Google-grounded daily limit reached. Switch off Google-grounded research to use standard research.")
	case errors.Is(e, postgres.ErrCapacity):
		fail(w, 429, "Research capacity or daily quota reached. Try later.")
	default:
		slog.Error("database operation failed")
		fail(w, 503, "Database unavailable. No new research was started.")
	}
}
func validID(w http.ResponseWriter, r *http.Request) bool {
	if _, e := uuid.Parse(r.PathValue("id")); e != nil {
		fail(w, 400, "Invalid identifier.")
		return false
	}
	return true
}
func (a *Server) create(w http.ResponseWriter, r *http.Request) {
	v, e := a.Store.CreateConversation(r.Context(), owner(r))
	if e != nil {
		a.problem(w, e)
		return
	}
	reply(w, 201, v)
}
func (a *Server) list(w http.ResponseWriter, r *http.Request) {
	v, e := a.Store.List(r.Context(), owner(r))
	if e != nil {
		a.problem(w, e)
		return
	}
	reply(w, 200, v)
}
func (a *Server) conversation(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	_ = a.Store.Recover(r.Context())
	v, e := a.Store.Conversation(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		a.problem(w, e)
		return
	}
	reply(w, 200, v)
}
func (a *Server) delete(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	e := a.Store.Delete(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		a.problem(w, e)
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}
func (a *Server) message(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	var v struct {
		Text             string `json:"text"`
		ResearchMode     string `json:"research_mode"`
		CostAcknowledged bool   `json:"cost_acknowledged"`
		RequestID        string `json:"request_id"`
		Version          int    `json:"version"`
	}
	if e := body(r, &v); e != nil {
		fail(w, 400, e.Error())
		return
	}
	if v.ResearchMode == "" {
		v.ResearchMode = "standard"
	}
	if v.ResearchMode != "standard" && v.ResearchMode != "google_grounded" {
		fail(w, 400, "Unknown research mode.")
		return
	}
	if v.ResearchMode == "google_grounded" {
		if !v.CostAcknowledged {
			fail(w, 400, "Acknowledge the Google-grounded cost warning before using this route.")
			return
		}
		if !a.Engine.GroundedReady() {
			fail(w, 503, "Google-grounded research is unavailable. Use standard research.")
			return
		}
	} else if !a.Engine.Ready() {
		fail(w, 503, "Configure Gemini and Tavily on the server before standard research.")
		return
	}
	refresh := strings.HasSuffix(r.URL.Path, "/refresh")
	v.Text = strings.TrimSpace(v.Text)
	if refresh {
		v.Text = "Refresh the sources with my existing requirements; preserve all mandatory constraints."
	}
	if v.Text == "" || len([]rune(v.Text)) > 1500 {
		fail(w, 400, "Message must contain 1–1500 characters.")
		return
	}
	if _, e := uuid.Parse(v.RequestID); e != nil {
		fail(w, 400, "A UUID request_id is required.")
		return
	}
	run, created, e := a.Store.Accept(r.Context(), owner(r), r.PathValue("id"), v.RequestID, v.Text, v.Version, refresh, a.Config.RunTimeout, a.Config.OwnerQuota, a.Config.GlobalQuota, postgres.ModeLimits{Mode: v.ResearchMode, OwnerQuota: a.Config.GroundedOwnerQuota, GlobalQuota: a.Config.GroundedGlobalQuota})
	if e != nil {
		a.problem(w, e)
		return
	}
	if created {
		a.Engine.Start(owner(r), run, v.Text, refresh)
	}
	reply(w, 202, run)
}
func (a *Server) run(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	_ = a.Store.Recover(r.Context())
	v, e := a.Store.Run(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		a.problem(w, e)
		return
	}
	reply(w, 200, v)
}
func (a *Server) cancel(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	e := a.Store.Cancel(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		a.problem(w, e)
		return
	}
	a.Engine.Cancel(r.PathValue("id"))
	reply(w, 200, map[string]bool{"ok": true})
}
func (a *Server) events(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	if _, e := a.Store.Run(r.Context(), owner(r), r.PathValue("id")); e != nil {
		a.problem(w, e)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "Streaming unavailable.")
		return
	}
	after, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if after == 0 {
		after, _ = strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	flusher.Flush()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, ok := a.identity(r); !ok {
			return
		}
		events, e := a.Store.Events(r.Context(), r.PathValue("id"), after)
		if e != nil {
			return
		}
		for _, v := range events {
			b, _ := json.Marshal(v)
			if _, e = fmt.Fprintf(w, "id: %d\nevent: progress\ndata: %s\n\n", v.Sequence, b); e != nil {
				return
			}
			after = v.Sequence
		}
		flusher.Flush()
		run, e := a.Store.Run(r.Context(), owner(r), r.PathValue("id"))
		if e != nil {
			return
		}
		if run.Status != "running" {
			fmt.Fprint(w, "event: done\ndata: {}\n\n")
			flusher.Flush()
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// Provider HTML is isolated from the app DOM, authenticated, and cannot run scripts.
func (a *Server) groundingWidget(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	run, err := a.Store.Run(r.Context(), owner(r), r.PathValue("id"))
	if err != nil {
		a.problem(w, err)
		return
	}
	if run.Result == nil || run.Result.Grounding == nil || run.Result.Grounding.SearchSuggestions == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src https: data:; script-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'; sandbox allow-popups allow-popups-to-escape-sandbox")
	_, _ = io.WriteString(w, run.Result.Grounding.SearchSuggestions)
}
