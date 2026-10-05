package postgres_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/storage/postgres"
	"table-for-you/backend/internal/testutil"
	"testing"
	"time"
)

func TestRunHistoryPaginationOwnershipAndRetention(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	token, err := s.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.Owner(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	chat, err := s.CreateConversation(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 23; i++ {
		run, _, e := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "query", i, false, time.Minute, 100, 100, postgres.ModeLimits{Mode: "standard"})
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Finish(ctx, run, "completed", "", &domain.Result{}, domain.Usage{}); e != nil {
			t.Fatal(e)
		}
	}
	a, err := s.ConversationRuns(ctx, owner, chat.ID, "")
	if err != nil || len(a.Runs) != 20 || a.Next == "" {
		t.Fatalf("first page: %+v %v", a, err)
	}
	b, err := s.ConversationRuns(ctx, owner, chat.ID, a.Next)
	if err != nil || len(b.Runs) != 3 || b.Next != "" {
		t.Fatalf("second page: %+v %v", b, err)
	}
	seen := map[string]bool{}
	for _, r := range append(a.Runs, b.Runs...) {
		if seen[r.ID] {
			t.Fatal("duplicate history run")
		}
		seen[r.ID] = true
	}
	if _, err = s.ConversationRuns(ctx, uuid.NewString(), chat.ID, ""); !errors.Is(err, postgres.ErrNotFound) {
		t.Fatal("foreign history exposed")
	}
	if _, err = s.ConversationRuns(ctx, owner, chat.ID, "invalid"); err == nil {
		t.Fatal("bad cursor accepted")
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE conversations SET updated_at=now()-interval '8 days' WHERE id=$1", chat.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConversationRuns(ctx, owner, chat.ID, ""); !errors.Is(err, postgres.ErrNotFound) {
		t.Fatal("expired history exposed")
	}
}
