package testutil

import (
	"context"
	"github.com/google/uuid"
	"nebulaiq/internal/storage/postgres"
	"net/url"
	"os"
	"testing"
)

func Database(t *testing.T) *postgres.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; PostgreSQL integration test skipped")
	}
	ctx := context.Background()
	base, e := postgres.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "test_" + uuid.New().String()[:8]
	if _, e = base.Pool.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		base.Close()
		t.Fatal(e)
	}
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := postgres.Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		s.Close()
		base.Pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		base.Close()
	})
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	return s
}
