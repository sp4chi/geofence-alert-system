// Package store holds all Postgres access. Nothing outside this package writes SQL.
package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a connection pool and waits until the database answers.
// Retrying matters on cloud hosts, where the API container can boot before Postgres is ready.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	const attempts = 15
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if attempt == attempts {
			pool.Close()
			return nil, fmt.Errorf("ping database after %d attempts: %w", attempts, err)
		}
		slog.Info("database not ready, retrying", "attempt", attempt, "err", err)

		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
