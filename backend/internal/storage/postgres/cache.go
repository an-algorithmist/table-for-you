package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"table-for-you/backend/internal/domain"
)

// Search reads an owner-scoped, unexpired cached search result.
func (s *Store) Search(ctx context.Context, owner, key string) ([]domain.SearchHit, bool, error) {
	var b []byte
	err := s.Pool.QueryRow(ctx, "SELECT payload FROM search_cache WHERE owner_id=$1 AND request_hash=$2 AND expires_at>now()", owner, key).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, persistenceError("search", err)
	}
	var hits []domain.SearchHit
	err = json.Unmarshal(b, &hits)
	return hits, err == nil, persistenceError("search", err)
}

// PutSearch caches owner-scoped search results for six hours.
func (s *Store) PutSearch(ctx context.Context, owner, key string, hits []domain.SearchHit) error {
	b, err := json.Marshal(hits)
	if err != nil {
		return persistenceError("put search", err)
	}
	_, err = s.Pool.Exec(ctx, "INSERT INTO search_cache(owner_id,request_hash,payload,fetched_at,expires_at) VALUES($1,$2,$3,now(),now()+interval '6 hours') ON CONFLICT(owner_id,request_hash) DO UPDATE SET payload=excluded.payload,fetched_at=excluded.fetched_at,expires_at=excluded.expires_at", owner, key, b)
	return persistenceError("put search", err)
}

// Document reads the latest unexpired document and updates its last-use timestamp.
func (s *Store) Document(ctx context.Context, key string) (domain.Document, bool, error) {
	var d domain.Document
	var b []byte
	err := s.Pool.QueryRow(ctx, "UPDATE source_documents SET last_used_at=now() WHERE id=(SELECT id FROM source_documents WHERE lookup_hash=$1 AND expires_at>now() ORDER BY fetched_at DESC LIMIT 1) RETURNING payload", key).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, false, nil
	}
	if err != nil {
		return d, false, persistenceError("document", err)
	}
	err = json.Unmarshal(b, &d)
	return d, err == nil, persistenceError("document", err)
}

// PutDocument stores a fetched document with its evidence timestamps.
func (s *Store) PutDocument(ctx context.Context, key string, d domain.Document) error {
	b, err := json.Marshal(d)
	if err != nil {
		return persistenceError("put document", err)
	}
	_, err = s.Pool.Exec(ctx, "INSERT INTO source_documents(id,lookup_hash,url,content_hash,payload,fetched_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)", d.ID, key, d.URL, d.Hash, b, d.FetchedAt, d.ExpiresAt)
	return persistenceError("put document", err)
}

// Extraction reads a parsed menu snapshot and updates its last-use timestamp.
func (s *Store) Extraction(ctx context.Context, key string) (domain.Extraction, bool, error) {
	var v domain.Extraction
	var b []byte
	err := s.Pool.QueryRow(ctx, "UPDATE extractions SET last_used_at=now() WHERE request_hash=$1 RETURNING payload", key).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, false, nil
	}
	if err != nil {
		return v, false, persistenceError("extraction", err)
	}
	err = json.Unmarshal(b, &v)
	return v, err == nil, persistenceError("extraction", err)
}

// PutExtraction replaces the parsed snapshot for a compatible request hash.
func (s *Store) PutExtraction(ctx context.Context, key string, v domain.Extraction) error {
	b, err := json.Marshal(v)
	if err != nil {
		return persistenceError("put extraction", err)
	}
	_, err = s.Pool.Exec(ctx, "INSERT INTO extractions(request_hash,payload) VALUES($1,$2) ON CONFLICT(request_hash) DO UPDATE SET payload=excluded.payload,last_used_at=now()", key, b)
	return persistenceError("put extraction", err)
}

// DocTTL separates shorter-lived review evidence from menu document caching.
func DocTTL(kind string) time.Duration {
	if kind == "review" {
		return 6 * time.Hour
	}
	return 24 * time.Hour
}
