package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Hash derives a one-way lookup key; browser session secrets are never persisted directly.
func Hash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }

// NewSession atomically creates a guest owner and a seven-day browser credential.
func (s *Store) NewSession(ctx context.Context) (token string, err error) {
	token = uuid.NewString() + uuid.NewString()
	owner := uuid.NewString()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", persistenceError("new session", err)
	}
	defer rollbackTransaction(tx)
	if _, err = tx.Exec(ctx, "INSERT INTO owners(id) VALUES($1)", owner); err != nil {
		return "", persistenceError("new session", err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO browser_sessions(id,owner_id,token_hash,expires_at) VALUES($1,$2,$3,now()+interval '7 days')", uuid.NewString(), owner, Hash(token)); err != nil {
		return "", persistenceError("new session", err)
	}
	return token, tx.Commit(ctx)
}

// Owner resolves an unexpired, non-revoked browser credential to its owner.
func (s *Store) Owner(ctx context.Context, token string) (string, error) {
	var owner string
	err := s.Pool.QueryRow(ctx, "SELECT owner_id::text FROM browser_sessions WHERE token_hash=$1 AND expires_at>now() AND revoked_at IS NULL", Hash(token)).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return owner, persistenceError("owner", err)
}

// Revoke invalidates a credential without deleting its owner or conversation history.
func (s *Store) Revoke(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, "UPDATE browser_sessions SET revoked_at=now() WHERE token_hash=$1", Hash(token))
	return persistenceError("revoke", err)
}
