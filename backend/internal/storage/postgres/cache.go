package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"nebulaiq/internal/domain"
	"time"
)

func (s *Store) Search(ctx context.Context, owner, key string) ([]domain.SearchHit, bool, error) {
	var b []byte
	e := s.Pool.QueryRow(ctx, "SELECT payload FROM search_cache WHERE owner_id=$1 AND request_hash=$2 AND expires_at>now()", owner, key).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	var hits []domain.SearchHit
	e = json.Unmarshal(b, &hits)
	return hits, e == nil, e
}
func (s *Store) PutSearch(ctx context.Context, owner, key string, hits []domain.SearchHit) error {
	b, e := json.Marshal(hits)
	if e != nil {
		return e
	}
	_, e = s.Pool.Exec(ctx, "INSERT INTO search_cache(owner_id,request_hash,payload,fetched_at,expires_at) VALUES($1,$2,$3,now(),now()+interval '6 hours') ON CONFLICT(owner_id,request_hash) DO UPDATE SET payload=excluded.payload,fetched_at=excluded.fetched_at,expires_at=excluded.expires_at", owner, key, b)
	return e
}
func (s *Store) Document(ctx context.Context, key string) (domain.Document, bool, error) {
	var d domain.Document
	var b []byte
	e := s.Pool.QueryRow(ctx, "UPDATE source_documents SET last_used_at=now() WHERE id=(SELECT id FROM source_documents WHERE lookup_hash=$1 AND expires_at>now() ORDER BY fetched_at DESC LIMIT 1) RETURNING payload", key).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		return d, false, nil
	}
	if e != nil {
		return d, false, e
	}
	e = json.Unmarshal(b, &d)
	return d, e == nil, e
}
func (s *Store) PutDocument(ctx context.Context, key string, d domain.Document) error {
	b, e := json.Marshal(d)
	if e != nil {
		return e
	}
	_, e = s.Pool.Exec(ctx, "INSERT INTO source_documents(id,lookup_hash,url,content_hash,payload,fetched_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)", d.ID, key, d.URL, d.Hash, b, d.FetchedAt, d.ExpiresAt)
	return e
}
func (s *Store) Extraction(ctx context.Context, key string) (domain.Extraction, bool, error) {
	var v domain.Extraction
	var b []byte
	e := s.Pool.QueryRow(ctx, "UPDATE extractions SET last_used_at=now() WHERE request_hash=$1 RETURNING payload", key).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, false, nil
	}
	if e != nil {
		return v, false, e
	}
	e = json.Unmarshal(b, &v)
	return v, e == nil, e
}
func (s *Store) PutExtraction(ctx context.Context, key string, v domain.Extraction) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = s.Pool.Exec(ctx, "INSERT INTO extractions(request_hash,payload) VALUES($1,$2) ON CONFLICT(request_hash) DO UPDATE SET payload=excluded.payload,last_used_at=now()", key, b)
	return e
}
func DocTTL(kind string) time.Duration {
	if kind == "review" {
		return 6 * time.Hour
	}
	return 24 * time.Hour
}
