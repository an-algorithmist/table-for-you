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

func TestOwnershipIdempotencyHistoryAndRecovery(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	token, e := s.NewSession(ctx)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := s.Owner(ctx, token)
	if e != nil {
		t.Fatal(e)
	}
	token2, _ := s.NewSession(ctx)
	other, _ := s.Owner(ctx, token2)
	c, e := s.CreateConversation(ctx, owner)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Conversation(ctx, other, c.ID); !errors.Is(e, postgres.ErrNotFound) {
		t.Fatal("other owner read conversation")
	}
	request := uuid.NewString()
	r, created, e := s.Accept(ctx, owner, c.ID, request, "vegan lunch in Barcelona", 0, false, time.Minute, 20, 100)
	if e != nil || !created {
		t.Fatal(e)
	}
	again, created, e := s.Accept(ctx, owner, c.ID, request, "vegan lunch in Barcelona", 0, false, time.Minute, 20, 100)
	if e != nil || created || again.ID != r.ID {
		t.Fatal("duplicate request was not idempotent")
	}
	if _, _, e = s.Accept(ctx, owner, c.ID, uuid.NewString(), "another", 1, false, time.Minute, 20, 100); !errors.Is(e, postgres.ErrConflict) {
		t.Fatal("concurrent conversation run accepted")
	}
	req := domain.Requirements{City: "Barcelona", Country: "Spain", Meal: "lunch", Diet: "vegan"}
	if e = s.Requirements(ctx, r, req); e != nil {
		t.Fatal(e)
	}
	result := &domain.Result{Requirements: req, Confirmed: []domain.Restaurant{}, Sources: []domain.Document{{ID: uuid.NewString(), URL: "https://restaurant.example/menu", Text: "immutable evidence"}}}
	if e = s.Finish(ctx, r, "completed", "", result, domain.Usage{Searches: 1}); e != nil {
		t.Fatal(e)
	}
	reopened, e := s.Conversation(ctx, owner, c.ID)
	if e != nil || len(reopened.Messages) != 2 || len(reopened.Runs) != 1 || reopened.Runs[0].Result.Sources[0].Text != "immutable evidence" {
		t.Fatalf("history not persisted: %v", e)
	}
	r2, _, e := s.Accept(ctx, owner, c.ID, uuid.NewString(), "followup", 1, false, time.Minute, 20, 100)
	if e != nil {
		t.Fatal(e)
	}
	s.Pool.Exec(ctx, "UPDATE runs SET lease_until=now()-interval '1 second' WHERE id=$1", r2.ID)
	if e = s.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	rr, e := s.Run(ctx, owner, r2.ID)
	if e != nil || rr.Status != "interrupted" {
		t.Fatal("expired run was not interrupted")
	}
	s.Revoke(ctx, token)
	if _, e = s.Owner(ctx, token); !errors.Is(e, postgres.ErrNotFound) {
		t.Fatal("revoked session still authorized")
	}
	var n int
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM conversations WHERE id=$1", c.ID).Scan(&n)
	if n != 1 {
		t.Fatal("session revocation deleted history")
	}
}
func TestPersistentCacheExpiryAndSnapshots(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	h := []domain.SearchHit{{URL: "https://restaurant.example", Content: "menu", FetchedAt: time.Now().UTC()}}
	if e := s.PutSearch(ctx, owner, "key", h); e != nil {
		t.Fatal(e)
	}
	got, hit, e := s.Search(ctx, owner, "key")
	if e != nil || !hit || len(got) != 1 {
		t.Fatal("cache hit failed")
	}
	s.Pool.Exec(ctx, "UPDATE search_cache SET expires_at=now()-interval '1 second'")
	_, hit, e = s.Search(ctx, owner, "key")
	if e != nil || hit {
		t.Fatal("expired search reused")
	}
	now := time.Now().UTC()
	d := domain.Document{ID: uuid.NewString(), URL: "https://restaurant.example/menu", Text: "menu v1", Hash: "hash1", FetchedAt: now, ExpiresAt: now.Add(time.Hour)}
	if e = s.PutDocument(ctx, "doc-key", d); e != nil {
		t.Fatal(e)
	}
	old, hit, e := s.Document(ctx, "doc-key")
	if e != nil || !hit || old.Hash != "hash1" {
		t.Fatal("document lookup failed")
	}
	d.ID = uuid.NewString()
	d.Text = "menu v2"
	d.Hash = "hash2"
	d.FetchedAt = now.Add(time.Second)
	if e = s.PutDocument(ctx, "doc-key", d); e != nil {
		t.Fatal(e)
	}
	newDoc, hit, e := s.Document(ctx, "doc-key")
	if e != nil || !hit || newDoc.Hash != "hash2" {
		t.Fatal("document version not updated")
	}
	var n int
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM source_documents").Scan(&n)
	if n != 2 {
		t.Fatal("historical source was overwritten")
	}
}

func TestDeletingHistoryDoesNotResetDailyQuota(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	chat, _ := s.CreateConversation(ctx, owner)
	run, _, e := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "one", 0, false, time.Minute, 1, 100)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Finish(ctx, run, "failed", "controlled failure", nil, domain.Usage{}); e != nil {
		t.Fatal(e)
	}
	if e = s.Delete(ctx, owner, chat.ID); e != nil {
		t.Fatal(e)
	}
	chat, _ = s.CreateConversation(ctx, owner)
	if _, _, e = s.Accept(ctx, owner, chat.ID, uuid.NewString(), "two", 0, false, time.Minute, 1, 100); !errors.Is(e, postgres.ErrCapacity) {
		t.Fatal("deleting history bypassed quota")
	}
}
