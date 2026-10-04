package postgres

import (
	"context"
)

// Recover interrupts runs whose worker lease has expired; it never replays provider calls.
func (s *Store) Recover(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, "UPDATE runs SET status='interrupted',error='Service interrupted; retry explicitly.',finished_at=now() WHERE status='running' AND lease_until<now()")
	return persistenceError("recover", err)
}

// Cleanup deletes bounded batches of expired history and cache entries.
func (s *Store) Cleanup(ctx context.Context) error {
	queries := []string{
		"DELETE FROM conversations WHERE id IN(SELECT id FROM conversations WHERE updated_at<now()-interval '7 days' AND NOT EXISTS(SELECT 1 FROM runs WHERE conversation_id=conversations.id AND status='running') LIMIT 100)",
		"DELETE FROM browser_sessions WHERE id IN(SELECT id FROM browser_sessions WHERE expires_at<now() OR revoked_at IS NOT NULL LIMIT 100)",
		"DELETE FROM search_cache WHERE (owner_id,request_hash) IN(SELECT owner_id,request_hash FROM search_cache WHERE expires_at<now() LIMIT 100)",
		"DELETE FROM source_documents WHERE id IN(SELECT id FROM source_documents WHERE last_used_at<now()-interval '7 days' OR (SELECT coalesce(sum(pg_column_size(payload)),0) FROM source_documents)>104857600 ORDER BY last_used_at LIMIT 50)",
		"DELETE FROM extractions WHERE request_hash IN(SELECT request_hash FROM extractions WHERE last_used_at<now()-interval '7 days' OR (SELECT coalesce(sum(pg_column_size(payload)),0) FROM extractions)>104857600 ORDER BY last_used_at LIMIT 50)",
	}
	for _, q := range queries {
		if _, err := s.Pool.Exec(ctx, q); err != nil {
			return persistenceError("cleanup", err)
		}
	}
	return nil
}
