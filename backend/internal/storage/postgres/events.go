package postgres

import (
	"context"

	"table-for-you/backend/internal/domain"
)

// Event appends a trace event. A run has one event-writing pipeline owner.
func (s *Store) Event(ctx context.Context, run, kind, message string) error {
	_, err := s.Pool.Exec(ctx, "INSERT INTO run_events(run_id,sequence,type,message) SELECT $1,coalesce(max(sequence),0)+1,$2,$3 FROM run_events WHERE run_id=$1", run, kind, message)
	return persistenceError("event", err)
}

// Events returns the next bounded batch of persisted events for SSE cursor replay.
func (s *Store) Events(ctx context.Context, run string, after int64) ([]domain.Event, error) {
	rows, err := s.Pool.Query(ctx, "SELECT sequence,type,message,created_at FROM run_events WHERE run_id=$1 AND sequence>$2 ORDER BY sequence LIMIT 200", run, after)
	if err != nil {
		return nil, persistenceError("events", err)
	}
	defer rows.Close()
	out := []domain.Event{}
	for rows.Next() {
		var v domain.Event
		if err = rows.Scan(&v.Sequence, &v.Type, &v.Message, &v.CreatedAt); err != nil {
			return nil, persistenceError("events", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
