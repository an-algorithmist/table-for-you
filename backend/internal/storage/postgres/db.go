package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"table-for-you/backend/migrations"
)

// Store owns the PostgreSQL repository boundary. SQL remains parameterized;
// retaining pgx preserves the existing transactional admission and lease semantics.
type Store struct{ Pool *pgxpool.Pool }

// Open creates and verifies a bounded PostgreSQL connection pool.
func Open(ctx context.Context, url string) (*Store, error) {
	c, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	c.MaxConns = 5
	c.ConnConfig.ConnectTimeout = 5 * time.Second
	p, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, persistenceError("open", err)
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, errors.New("PostgreSQL connection failed")
	}
	return &Store{p}, nil
}

// Close releases all connections owned by the store.
func (s *Store) Close() { s.Pool.Close() }

// Migrate applies embedded SQL migrations in filename order under an advisory lock.
func (s *Store) Migrate(ctx context.Context) error {
	c, err := s.Pool.Acquire(ctx)
	if err != nil {
		return persistenceError("migrate", err)
	}
	defer c.Release()
	if _, err = c.Exec(ctx, "SELECT pg_advisory_lock(918372)"); err != nil {
		return persistenceError("migrate", err)
	}
	defer releaseMigrationLock(c)
	if _, err = c.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return persistenceError("migrate", err)
	}
	files, err := migrations.Files.ReadDir(".")
	if err != nil {
		return persistenceError("migrate", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		var done bool
		if err = c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", f.Name()).Scan(&done); err != nil {
			return persistenceError("migrate", err)
		}
		if done {
			continue
		}
		b, err := migrations.Files.ReadFile(f.Name())
		if err != nil {
			return persistenceError("migrate", err)
		}
		tx, err := c.Begin(ctx)
		if err != nil {
			return persistenceError("migrate", err)
		}
		if _, err = tx.Exec(ctx, string(b)); err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", f.Name())
		}
		if err != nil {
			rollbackTransaction(tx)
			return fmt.Errorf("migration %s failed: %w", f.Name(), err)
		}
		if err = tx.Commit(ctx); err != nil {
			return persistenceError("migrate", err)
		}
	}
	return nil
}
