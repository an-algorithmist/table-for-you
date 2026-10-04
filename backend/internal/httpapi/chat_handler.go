package httpapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"table-for-you/backend/internal/storage/postgres"
)

// messageRequest is the JSON admission contract; research results use domain.Run.
type messageRequest struct {
	Text             string `json:"text"`
	ResearchMode     string `json:"research_mode"`
	CostAcknowledged bool   `json:"cost_acknowledged"`
	RequestID        string `json:"request_id"`
	Version          int    `json:"version"`
}

func (server *Server) message(w http.ResponseWriter, r *http.Request) {
	if !validID(w, r) {
		return
	}
	var input messageRequest
	if err := body(r, &input); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.ResearchMode == "" {
		input.ResearchMode = "standard"
	}
	if input.ResearchMode != "standard" && input.ResearchMode != "google_grounded" {
		fail(w, http.StatusBadRequest, "Unknown research mode.")
		return
	}
	if input.ResearchMode == "google_grounded" {
		if !input.CostAcknowledged {
			fail(w, http.StatusBadRequest, "Acknowledge the Google-grounded cost warning before using this route.")
			return
		}
		if !server.Engine.GroundedReady() {
			fail(w, http.StatusServiceUnavailable, "Google-grounded research is unavailable. Use standard research.")
			return
		}
	} else if !server.Engine.Ready() {
		fail(w, http.StatusServiceUnavailable, "Configure Gemini and Tavily on the server before standard research.")
		return
	}
	refresh := strings.HasSuffix(r.URL.Path, "/refresh")
	input.Text = strings.TrimSpace(input.Text)
	if refresh {
		input.Text = "Refresh the sources with my existing requirements; preserve all mandatory constraints."
	}
	if input.Text == "" || len([]rune(input.Text)) > 1500 {
		fail(w, http.StatusBadRequest, "Message must contain 1–1500 characters.")
		return
	}
	if _, err := uuid.Parse(input.RequestID); err != nil {
		fail(w, http.StatusBadRequest, "A UUID request_id is required.")
		return
	}
	run, created, err := server.Store.Accept(
		r.Context(), owner(r), r.PathValue("id"), input.RequestID, input.Text,
		input.Version, refresh, server.Config.RunTimeout, server.Config.OwnerQuota, server.Config.GlobalQuota,
		postgres.ModeLimits{
			Mode:        input.ResearchMode,
			OwnerQuota:  server.Config.GroundedOwnerQuota,
			GlobalQuota: server.Config.GroundedGlobalQuota,
		},
	)
	if err != nil {
		server.problem(w, err)
		return
	}
	if created {
		server.Engine.Start(owner(r), run, input.Text, refresh)
	}
	reply(w, http.StatusAccepted, run)
}
