package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/maxaixs/mandat/backend/internal/errs"
)

// uniqueViolation is the PostgreSQL SQLSTATE for unique constraint violations.
const uniqueViolation = "23505"

// mapError translates PostgreSQL errors into domain sentinel errors.
// Callers pass their own not-found sentinel since it varies by aggregate.
func mapError(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return fmt.Errorf("%s: %w", pgErr.ConstraintName, errs.ErrConflict)
	}

	return err
}
