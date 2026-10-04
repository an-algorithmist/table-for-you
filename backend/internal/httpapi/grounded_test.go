package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/testutil"
)

func TestGroundedAcknowledgementAndWidgetOwnership(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	chat, _ := s.CreateConversation(ctx, owner)
	cfg := config.Config{Local: true}
	engine := inertResearch{}
	server := httptest.NewServer(New(s, engine, cfg))
	defer server.Close()
	request := func(method, path, payload, session string) *http.Response {
		t.Helper()
		r, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString(payload))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Nebula-Request", "1")
		r.AddCookie(&http.Cookie{Name: "nebula_session", Value: session})
		res, err := server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := request("POST", "/api/conversations/"+chat.ID+"/messages", `{"text":"Tokyo dinner","research_mode":"google_grounded","request_id":"`+uuid.NewString()+`","version":0}`, token)
	if res.StatusCode != 400 {
		t.Fatalf("missing cost acknowledgement accepted: %d", res.StatusCode)
	}
	res.Body.Close()
	res = request("POST", "/api/conversations/"+chat.ID+"/messages", `{"text":"Tokyo dinner","research_mode":"invalid","request_id":"`+uuid.NewString()+`","version":0}`, token)
	if res.StatusCode != 400 {
		t.Fatal("unknown mode accepted")
	}
	res.Body.Close()
	run, _, err := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "Tokyo dinner", 0, false, time.Minute, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	html := `<style>div{color:blue}</style><div>Google suggestions</div><script>parent.alert(1)</script>`
	if err = s.Finish(ctx, run, "completed", "", &domain.Result{Answer: "Grounded", Grounding: &domain.Grounding{Text: "Grounded", SearchSuggestions: html}}, run.Usage); err != nil {
		t.Fatal(err)
	}
	res = request("GET", "/api/runs/"+run.ID+"/grounding-widget", "", token)
	content, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(content) != html || !strings.Contains(res.Header.Get("Content-Security-Policy"), "script-src 'none'") || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("widget isolation failed")
	}
	other, _ := s.NewSession(ctx)
	res = request("GET", "/api/runs/"+run.ID+"/grounding-widget", "", other)
	if res.StatusCode != 404 {
		t.Fatal("other owner accessed grounding widget")
	}
	res.Body.Close()
}
