package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"table-for-you/backend/internal/domain"
)

// CreateConversation creates an empty conversation belonging to owner.
func (s *Store) CreateConversation(ctx context.Context, owner string) (domain.Conversation, error) {
	v := domain.Conversation{ID: uuid.NewString(), Title: "New dining research", Requirements: domain.Requirements{}, UpdatedAt: time.Now().UTC()}
	_, err := s.Pool.Exec(ctx, "INSERT INTO conversations(id,owner_id) VALUES($1,$2)", v.ID, owner)
	return v, persistenceError("create conversation", err)
}

// List returns the owner's most recently updated conversations within the retention window.
func (s *Store) List(ctx context.Context, owner string) ([]domain.Conversation, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id::text,title,requirements,version,updated_at FROM conversations WHERE owner_id=$1 AND updated_at>now()-interval '7 days' ORDER BY updated_at DESC LIMIT 50", owner)
	if err != nil {
		return nil, persistenceError("list", err)
	}
	defer rows.Close()
	out := []domain.Conversation{}
	for rows.Next() {
		var v domain.Conversation
		var b []byte
		if err = rows.Scan(&v.ID, &v.Title, &b, &v.Version, &v.UpdatedAt); err != nil {
			return nil, persistenceError("list", err)
		}
		if err = json.Unmarshal(b, &v.Requirements); err != nil {
			return nil, persistenceError("list", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Conversation loads an owner-scoped conversation, its messages, and recent runs.
func (s *Store) Conversation(ctx context.Context, owner, id string) (domain.Conversation, error) {
	var v domain.Conversation
	var b []byte
	err := s.Pool.QueryRow(ctx, "SELECT id::text,title,requirements,version,updated_at FROM conversations WHERE id=$1 AND owner_id=$2 AND updated_at>now()-interval '7 days'", id, owner).Scan(&v.ID, &v.Title, &b, &v.Version, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, persistenceError("conversation", err)
	}
	if err = json.Unmarshal(b, &v.Requirements); err != nil {
		return v, persistenceError("conversation", err)
	}
	rows, err := s.Pool.Query(ctx, "SELECT id::text,role,text,created_at FROM messages WHERE conversation_id=$1 ORDER BY sequence", id)
	if err != nil {
		return v, persistenceError("conversation", err)
	}
	for rows.Next() {
		var m domain.Message
		if err = rows.Scan(&m.ID, &m.Role, &m.Text, &m.CreatedAt); err != nil {
			rows.Close()
			return v, persistenceError("conversation", err)
		}
		v.Messages = append(v.Messages, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, persistenceError("conversation", err)
	}
	rows, err = s.Pool.Query(ctx, "SELECT id::text,conversation_id::text,status,requirements,result,error,created_at,usage FROM runs WHERE conversation_id=$1 ORDER BY created_at DESC LIMIT 10", id)
	if err != nil {
		return v, persistenceError("conversation", err)
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return v, persistenceError("conversation", err)
		}
		v.Runs = append(v.Runs, r)
	}
	return v, rows.Err()
}

// Delete removes an owned conversation only when it has no active research.
func (s *Store) Delete(ctx context.Context, owner, id string) error {
	tag, err := s.Pool.Exec(ctx, "DELETE FROM conversations WHERE id=$1 AND owner_id=$2 AND NOT EXISTS(SELECT 1 FROM runs WHERE conversation_id=$1 AND status='running')", id, owner)
	if err != nil {
		return persistenceError("delete", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}
