package httpapi

import (
	"crypto/subtle"
	"net"
	"net/http"
	"time"

	"table-for-you/backend/internal/storage/postgres"
)

func (server *Server) allowAttempt(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	now := time.Now()
	recent := []time.Time{}
	for _, t := range server.attempts[host] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		return false
	}
	recent = append(recent, now)
	if len(server.attempts) > 1000 {
		server.attempts = map[string][]time.Time{}
	}
	server.attempts[host] = recent
	return true
}

func (server *Server) access(w http.ResponseWriter, r *http.Request) {
	if !server.allowAttempt(r) {
		fail(w, http.StatusTooManyRequests, "Too many access attempts; wait one minute.")
		return
	}
	var v struct {
		Code string `json:"code"`
	}
	if e := body(r, &v); e != nil {
		fail(w, http.StatusBadRequest, e.Error())
		return
	}
	expected, provided := postgres.Hash(server.Config.AccessCode), postgres.Hash(v.Code)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		fail(w, http.StatusUnauthorized, "Access code not accepted.")
		return
	}
	if _, ok := server.identity(r); ok {
		reply(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	token, e := server.Store.NewSession(r.Context())
	if e != nil {
		server.problem(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "nebula_session", Value: token, Path: "/", HttpOnly: true, Secure: !server.Config.Local, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 3600})
	reply(w, http.StatusOK, map[string]bool{"ok": true})
}

func (server *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("nebula_session")
	if err != nil {
		fail(w, http.StatusUnauthorized, "Your browser session expired. Open the dining notebook again.")
		return
	}
	if e := server.Store.Revoke(r.Context(), cookie.Value); e != nil {
		server.problem(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "nebula_session", Value: "", Path: "/", HttpOnly: true, Secure: !server.Config.Local, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	reply(w, http.StatusOK, map[string]bool{"ok": true})
}
