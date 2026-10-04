package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"table-for-you/backend/internal/domain"
)

// scanRun maps nullable result JSON and usage into the public run contract.
func scanRun(row pgx.Row) (domain.Run, error) {
	var r domain.Run
	var req, res, use []byte
	err := row.Scan(&r.ID, &r.ConversationID, &r.Status, &req, &res, &r.Error, &r.CreatedAt, &use)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, persistenceError("scan run", err)
	}
	if err = json.Unmarshal(req, &r.Requirements); err != nil {
		return r, persistenceError("scan run", err)
	}
	if len(res) > 0 {
		if err = json.Unmarshal(res, &r.Result); err != nil {
			return r, persistenceError("scan run", err)
		}
	}
	err = json.Unmarshal(use, &r.Usage)
	return r, persistenceError("scan run", err)
}

// Run loads a research run through its conversation's ownership and retention checks.
func (s *Store) Run(ctx context.Context, owner, id string) (domain.Run, error) {
	return scanRun(s.Pool.QueryRow(ctx, "SELECT r.id::text,r.conversation_id::text,r.status,r.requirements,r.result,r.error,r.created_at,r.usage FROM runs r JOIN conversations c ON c.id=r.conversation_id WHERE r.id=$1 AND c.owner_id=$2 AND c.updated_at>now()-interval '7 days'", id, owner))
}

// Requirements atomically updates the running research and its conversation context.
func (s *Store) Requirements(ctx context.Context, r domain.Run, req domain.Requirements) error {
	b, err := json.Marshal(req)
	if err != nil {
		return persistenceError("requirements", err)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return persistenceError("requirements", err)
	}
	defer rollbackTransaction(tx)
	tag, err := tx.Exec(ctx, "UPDATE runs SET requirements=$2 WHERE id=$1 AND status='running'", r.ID, b)
	if err != nil {
		return persistenceError("requirements", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, "UPDATE conversations SET requirements=$2 WHERE id=$1", r.ConversationID, b); err != nil {
		return persistenceError("requirements", err)
	}
	return tx.Commit(ctx)
}

// Heartbeat renews an active run's lease and reports whether cancellation was requested.
func (s *Store) Heartbeat(ctx context.Context, run string) (bool, error) {
	var cancel bool
	err := s.Pool.QueryRow(ctx, "UPDATE runs SET lease_until=now()+interval '30 seconds' WHERE id=$1 AND status='running' RETURNING cancel_requested", run).Scan(&cancel)
	return cancel, persistenceError("heartbeat", err)
}

// Cancel requests cooperative cancellation of an owned active research run.
func (s *Store) Cancel(ctx context.Context, owner, run string) error {
	tag, err := s.Pool.Exec(ctx, "UPDATE runs SET cancel_requested=true WHERE id=$1 AND status='running' AND conversation_id IN(SELECT id FROM conversations WHERE owner_id=$2)", run, owner)
	if err != nil {
		return persistenceError("cancel", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
