package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// contactsTable is the contacts table, ordered by a deferred unique position.
const contactsTable = "contacts"

// ContactRepo persists contacts in PostgreSQL.
type ContactRepo struct {
	pool *pgxpool.Pool
}

// NewContactRepo creates a ContactRepo backed by pool.
func NewContactRepo(pool *pgxpool.Pool) *ContactRepo {
	return &ContactRepo{pool: pool}
}

// List returns all contacts ordered by position.
func (r *ContactRepo) List(ctx context.Context) ([]*models.Contact, error) {
	const fc = "contactRepo_List"

	rows, err := r.pool.Query(ctx,
		`SELECT id, type, value, label, position, created_at, updated_at FROM contacts ORDER BY position, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	defer rows.Close()

	contacts := []*models.Contact{}
	for rows.Next() {
		var contact models.Contact
		err := rows.Scan(
			&contact.ID,
			&contact.Type,
			&contact.Value,
			&contact.Label,
			&contact.Position,
			&contact.CreatedAt,
			&contact.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fc, err)
		}
		contacts = append(contacts, &contact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return contacts, nil
}

// Create inserts the contact after the last one and sets its Position.
// Concurrent creates racing for one position fail with errs.ErrConflict.
func (r *ContactRepo) Create(ctx context.Context, contact *models.Contact) error {
	const fc = "contactRepo_Create"

	query := `INSERT INTO contacts (id, type, value, label, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, (SELECT COALESCE(MAX(position), 0) + 1 FROM contacts), $5, $6)
		RETURNING position`

	err := r.pool.QueryRow(ctx, query,
		contact.ID, contact.Type, contact.Value, contact.Label, contact.CreatedAt, contact.UpdatedAt,
	).Scan(&contact.Position)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrContactNotFound))
	}

	return nil
}

// Update saves type, value, label and updated_at and reads back Position and CreatedAt;
// missing id is errs.ErrContactNotFound.
func (r *ContactRepo) Update(ctx context.Context, contact *models.Contact) error {
	const fc = "contactRepo_Update"

	query := `UPDATE contacts SET type = $2, value = $3, label = $4, updated_at = $5 WHERE id = $1
		RETURNING position, created_at`

	err := r.pool.QueryRow(ctx, query,
		contact.ID, contact.Type, contact.Value, contact.Label, contact.UpdatedAt,
	).Scan(&contact.Position, &contact.CreatedAt)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrContactNotFound))
	}

	return nil
}

// Delete removes the contact; missing id is errs.ErrContactNotFound.
func (r *ContactRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const fc = "contactRepo_Delete"

	tag, err := r.pool.Exec(ctx, `DELETE FROM contacts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fc, errs.ErrContactNotFound)
	}

	return nil
}

// Move swaps the contact with its neighbour above (up) or below in one transaction.
// No neighbour is a no-op; missing id is errs.ErrContactNotFound.
func (r *ContactRepo) Move(ctx context.Context, id uuid.UUID, up bool) error {
	const fc = "contactRepo_Move"

	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return swapWithNeighbour(ctx, tx, contactsTable, id, up)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrContactNotFound))
	}

	return nil
}
