package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"table-for-you/backend/internal/storage/postgres"
	"table-for-you/backend/internal/testutil"
)

// An error after mode quota charging must roll back every admission side effect.
// Otherwise a failed database write could consume the expensive-mode allowance.
func TestAdmissionWriteFailureRollsBackQuotaAndHistory(t *testing.T) {
	store := testutil.Database(t)
	ctx := context.Background()
	token, err := store.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Owner(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := store.CreateConversation(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Pool.Exec(ctx, "ALTER TABLE messages ADD CONSTRAINT injected_failure CHECK (text <> 'reject this admission')"); err != nil {
		t.Fatal(err)
	}
	limits := postgres.ModeLimits{Mode: "google_grounded", OwnerQuota: 1, GlobalQuota: 1}
	_, created, err := store.Accept(ctx, owner, conversation.ID, uuid.NewString(), "reject this admission", 0, false, time.Minute, 20, 100, limits)
	if err == nil || created {
		t.Fatalf("failed write accepted: created=%v err=%v", created, err)
	}
	restored, err := store.Conversation(ctx, owner, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Version != 0 || len(restored.Messages) != 0 || len(restored.Runs) != 0 {
		t.Fatalf("failed admission left history: %+v", restored)
	}
	// A fresh accepted request proves both owner and global mode counters rolled back.
	run, created, err := store.Accept(ctx, owner, conversation.ID, uuid.NewString(), "Tokyo dinner", 0, false, time.Minute, 20, 100, limits)
	if err != nil || !created || run.Usage.Mode != "google_grounded" {
		t.Fatalf("failed admission consumed quota: created=%v err=%v", created, err)
	}
}
