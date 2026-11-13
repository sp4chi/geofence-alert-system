package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Domain errors (match with errors.Is). The HTTP layer maps these to status codes in one place (httpapi/errors.go).
var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("already exists")
)

// domainError carries a client-safe message while still matching a sentinel via errors.Is.
type domainError struct {
	kind    error
	message string
}

func (e *domainError) Error() string { return e.message }
func (e *domainError) Unwrap() error { return e.kind }

// ClientMessage returns the client-safe message of a not-found/duplicate error, even when callers
// have wrapped it with internal context (e.g. "update location of veh_1: ...").
func ClientMessage(err error) string {
	var domain *domainError
	if errors.As(err, &domain) {
		return domain.message
	}
	return err.Error()
}

// InvalidGeometryError means PostGIS rejected a shape (e.g. a self-intersecting "bow-tie" polygon).
type InvalidGeometryError struct {
	Reason string // from ST_IsValidReason, e.g. "Self-intersection[-122.41 37.78]"
}

func (e *InvalidGeometryError) Error() string { return "invalid geometry: " + e.Reason }

// Store is the Postgres-backed data access layer. Safe for concurrent use (the pool is).
type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Ping reports whether the database answers; used by /healthz.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// isUniqueViolation reports whether err is Postgres error 23505 on the given constraint/index.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

func wrap(action string, err error) error {
	return fmt.Errorf("%s: %w", action, err)
}
