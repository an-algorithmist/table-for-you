package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cleanup uses its own deadline so a cancelled request cannot strand a transaction.
// Rollback after a successful commit is expected and is not an application failure.
func rollbackTransaction(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("rollback database transaction", "error", err)
	}
}

func releaseMigrationLock(conn *pgxpool.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock(918372)"); err != nil {
		slog.Error("release migration advisory lock", "error", err)
	}
}
