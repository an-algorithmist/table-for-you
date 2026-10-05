package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"table-for-you/backend/internal/domain"
	"time"
)

// RunPage is a bounded history page with a stable created-at/id cursor.
type RunPage struct {
	Runs []domain.Run `json:"runs"`
	Next string       `json:"next_cursor,omitempty"`
}
type runCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

// ConversationRuns checks ownership and retention before loading a historical page.
func (s *Store) ConversationRuns(ctx context.Context, owner, id, cursor string) (RunPage, error) {
	page := RunPage{Runs: []domain.Run{}}
	var owned bool
	if err := s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM conversations WHERE id=$1 AND owner_id=$2 AND updated_at>now()-interval '7 days')", id, owner).Scan(&owned); err != nil {
		return page, persistenceError("history ownership", err)
	}
	if !owned {
		return page, ErrNotFound
	}
	c := runCursor{At: time.Now().Add(24 * time.Hour), ID: "ffffffff-ffff-ffff-ffff-ffffffffffff"}
	if cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil {
			return page, fmt.Errorf("invalid history cursor")
		}
		if e = json.Unmarshal(b, &c); e != nil || c.ID == "" || c.At.IsZero() {
			return page, fmt.Errorf("invalid history cursor")
		}
	}
	if _, err := uuid.Parse(c.ID); err != nil {
		return page, fmt.Errorf("invalid history cursor")
	}
	rows, err := s.Pool.Query(ctx, "SELECT id::text,conversation_id::text,status,requirements,result,error,created_at,usage FROM runs WHERE conversation_id=$1 AND (created_at,id)<($2,$3::uuid) ORDER BY created_at DESC,id DESC LIMIT 21", id, c.At, c.ID)
	if err != nil {
		return page, persistenceError("history page", err)
	}
	defer rows.Close()
	for rows.Next() {
		r, e := scanRun(rows)
		if e != nil {
			return page, e
		}
		page.Runs = append(page.Runs, r)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Runs) > 20 {
		page.Runs = page.Runs[:20]
		last := page.Runs[19]
		b, _ := json.Marshal(runCursor{last.CreatedAt, last.ID})
		page.Next = base64.RawURLEncoding.EncodeToString(b)
	}
	return page, nil
}
