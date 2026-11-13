package store

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationLockID is an arbitrary constant; every API instance uses it so only one migrates at a time.
const migrationLockID = 727_001

// Migrate applies every *.sql file in files that hasn't run yet, in filename order.
// Each file runs in its own transaction and is recorded in schema_migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool, files fs.FS) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := applyMigration(ctx, pool, files, name); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, files fs.FS, name string) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Transaction-scoped lock: if two instances start together, the second waits here,
		// then sees the version already recorded and skips it.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLockID); err != nil {
			return err
		}

		var applied bool
		err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied)
		if err != nil || applied {
			return err
		}

		sql, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		// No arguments → pgx uses the simple protocol, which allows many statements in one Exec.
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			return err
		}
		slog.Info("applied migration", "version", name)
		return nil
	})
}
