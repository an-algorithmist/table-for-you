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

func TestGroundedQuotaModeIdempotencyAndDeletion(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	c, _ := s.CreateConversation(ctx, owner)
	limits := postgres.ModeLimits{Mode: "google_grounded", OwnerQuota: 1, GlobalQuota: 1}
	id := uuid.NewString()
	run, created, err := s.Accept(ctx, owner, c.ID, id, "Tokyo dinner", 0, false, time.Minute, 20, 100, limits)
	if err != nil || !created || run.Usage.Mode != "google_grounded" {
		t.Fatalf("mode admission: %v %+v", err, run)
	}
	same, created, err := s.Accept(ctx, owner, c.ID, id, "Tokyo dinner", 0, false, time.Minute, 20, 100, limits)
	if err != nil || created || same.ID != run.ID || same.Usage.Mode != "google_grounded" {
		t.Fatal("duplicate charged or mode lost")
	}
	s.Finish(ctx, run, "completed", "", &domain.Result{Answer: "Done"}, run.Usage)
	if err = s.Delete(ctx, owner, c.ID); err != nil {
		t.Fatal(err)
	}
	c, _ = s.CreateConversation(ctx, owner)
	_, _, err = s.Accept(ctx, owner, c.ID, uuid.NewString(), "Again", 0, false, time.Minute, 20, 100, limits)
	if !errors.Is(err, postgres.ErrGroundedCapacity) {
		t.Fatalf("deleting history bypassed grounded owner quota: %v", err)
	}
	token2, _ := s.NewSession(ctx)
	owner2, _ := s.Owner(ctx, token2)
	c2, _ := s.CreateConversation(ctx, owner2)
	_, _, err = s.Accept(ctx, owner2, c2.ID, uuid.NewString(), "Again", 0, false, time.Minute, 20, 100, limits)
	if !errors.Is(err, postgres.ErrGroundedCapacity) {
		t.Fatal("global grounded quota bypassed")
	}
	standard, created, err := s.Accept(ctx, owner, c.ID, uuid.NewString(), "Standard", 0, false, time.Minute, 20, 100)
	if err != nil || !created || standard.Usage.Mode != "standard" {
		t.Fatal("grounded quota affected standard flow")
	}
}
