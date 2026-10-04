package httpapi

import (
	"context"
	"net/http"
	"time"
)

func (server *Server) health(w http.ResponseWriter, r *http.Request) {
	reply(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) status(w http.ResponseWriter, r *http.Request) {
	_, authenticated := server.identity(r)
	reply(w, http.StatusOK, map[string]any{
		"authenticated":             authenticated,
		"local":                     server.Config.Local,
		"providers_ready":           server.Engine.Ready(),
		"model":                     server.Config.Model,
		"auth_stage":                "guest",
		"history_retention_days":    7,
		"google_grounded_available": server.Engine.GroundedReady(),
		"grounded_owner_limit":      server.Config.GroundedOwnerQuota,
		"grounded_global_limit":     server.Config.GroundedGlobalQuota,
	})
}

func (server *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, c := context.WithTimeout(r.Context(), 3*time.Second)
	defer c()
	if e := server.Store.Pool.Ping(ctx); e != nil {
		fail(w, http.StatusServiceUnavailable, "Database unavailable.")
		return
	}
	reply(w, http.StatusOK, map[string]string{"status": "ready"})
}
