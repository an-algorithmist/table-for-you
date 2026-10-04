package httpapi

import (
	"net/http"
)

func (server *Server) create(w http.ResponseWriter, r *http.Request) {
	v, e := server.Store.CreateConversation(r.Context(), owner(r))
	if e != nil {
		server.problem(w, e)
		return
	}
	reply(w, http.StatusCreated, v)
}

func (server *Server) list(w http.ResponseWriter, r *http.Request) {
	v, e := server.Store.List(r.Context(), owner(r))
	if e != nil {
		server.problem(w, e)
		return
	}
	reply(w, http.StatusOK, v)
}

func (server *Server) conversation(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	server.recoverRuns(r)
	v, e := server.Store.Conversation(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		server.problem(w, e)
		return
	}
	reply(w, http.StatusOK, v)
}

func (server *Server) delete(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	e := server.Store.Delete(r.Context(), owner(r), r.PathValue("id"))
	if e != nil {
		server.problem(w, e)
		return
	}
	reply(w, http.StatusOK, map[string]bool{"ok": true})
}
