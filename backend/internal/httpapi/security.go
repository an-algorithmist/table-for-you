package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"table-for-you/backend/internal/storage/postgres"
)

// Identity identifies the owner of server validated browser session.
type Identity struct{ OwnerID string }

// contextKey is deliberately private to prevent unrelated middleware replacing identity.
type contextKey struct{}

func (server *Server) boundaries(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != "GET" && r.Method != "HEAD" {
				if r.Header.Get("X-Nebula-Request") != "1" {
					fail(w, http.StatusForbidden, "Same-origin request header required.")
					return
				}
				if raw := r.Header.Get("Origin"); raw != "" {
					u, e := url.Parse(raw)
					if e != nil || u.Host != r.Host || (u.Scheme != "https" && u.Scheme != "http") {
						fail(w, http.StatusForbidden, "Cross-origin requests are not allowed.")
						return
					}
				}
			}
		}
		defer func() {
			if recover() != nil {
				slog.Error("HTTP handler panic", "path", r.URL.Path)
				fail(w, http.StatusInternalServerError, "Request failed.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (server *Server) identity(r *http.Request) (Identity, bool) {
	cookie, e := r.Cookie("nebula_session")
	if e != nil {
		return Identity{}, false
	}
	owner, e := server.Store.Owner(r.Context(), cookie.Value)
	return Identity{owner}, e == nil
}

func (server *Server) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, cookieError := r.Cookie("nebula_session")
		if cookieError != nil {
			fail(w, http.StatusUnauthorized, "Your browser session expired. Open the dining notebook again.")
			return
		}
		ownerID, err := server.Store.Owner(r.Context(), cookie.Value)
		if errors.Is(err, postgres.ErrNotFound) {
			fail(w, http.StatusUnauthorized, "Your browser session expired. Open the dining notebook again.")
			return
		}
		if err != nil {
			server.problem(w, err)
			return
		}
		v := Identity{OwnerID: ownerID}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, v)))
	})
}

func owner(r *http.Request) string { return r.Context().Value(contextKey{}).(Identity).OwnerID }
