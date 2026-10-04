package httpapi

import (
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

// No model is needed to reconnect to research already saved in PostgreSQL.
type inertResearch struct{}

func (inertResearch) Ready() bool                            { return false }
func (inertResearch) GroundedReady() bool                    { return false }
func (inertResearch) Start(string, domain.Run, string, bool) {}
func (inertResearch) Cancel(string)                          {}

func TestSSEReconnectReplaysOnlyEventsAfterCursor(t *testing.T) {
	store := testutil.Database(t)
	ctx := context.Background()
	token, err := store.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ownerID, err := store.Owner(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	chat, err := store.CreateConversation(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	run, _, err := store.Accept(ctx, ownerID, chat.ID, uuid.NewString(), "Barcelona lunch", 0, false, time.Minute, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Event(ctx, run.ID, "search.started", "first already received"); err != nil {
		t.Fatal(err)
	}
	if err := store.Event(ctx, run.ID, "menu.extracted", "second menu available"); err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, run, "completed", "", &domain.Result{Answer: "Saved recommendations"}, run.Usage); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(New(store, inertResearch{}, config.Config{Local: true}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 3 * time.Second
	for _, useHeader := range []bool{true, false} {
		name := "query cursor"
		if useHeader {
			name = "Last-Event-ID header"
		}
		t.Run(name, func(t *testing.T) {
			path := server.URL + "/api/runs/" + run.ID + "/events?after=1"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if useHeader {
				req.Header.Set("Last-Event-ID", "1")
				// A conflicting query must not override the EventSource header.
				req.URL.RawQuery = "after=2"
			}
			req.AddCookie(&http.Cookie{Name: "nebula_session", Value: token})
			res, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			stream, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			text := string(stream)
			if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
				t.Fatalf("unexpected stream response: %d", res.StatusCode)
			}
			if strings.Contains(text, "first already received") || !strings.Contains(text, "second menu available") || !strings.Contains(text, "Saved recommendations") || !strings.Contains(text, "event: done") {
				t.Fatalf("incorrect replay or missing completion: %s", text)
			}
		})
	}
}
