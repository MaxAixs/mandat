package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// adFormatColumns is the column list shared by ad format SELECT queries.
const adFormatColumns = `id, title, description, image_url, position, created_at, updated_at`

// adFormatsTable is the ad format table, ordered by a deferred unique position.
const adFormatsTable = "ad_formats"

// AdFormatRepo persists ad format cards in PostgreSQL.
type AdFormatRepo struct {
	pool *pgxpool.Pool
}

// NewAdFormatRepo creates an AdFormatRepo backed by pool.
func NewAdFormatRepo(pool *pgxpool.Pool) *AdFormatRepo {
	return &AdFormatRepo{pool: pool}
}

// List returns all cards ordered by position.
func (r *AdFormatRepo) List(ctx context.Context) ([]*models.AdFormat, error) {
	const fc = "adFormatRepo_List"

	rows, err := r.pool.Query(ctx, `SELECT `+adFormatColumns+` FROM ad_formats ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}
	defer rows.Close()

	formats := []*models.AdFormat{}
	for rows.Next() {
		format, err := scanAdFormat(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fc, err)
		}
		formats = append(formats, format)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return formats, nil
}

// GetByID returns one card or errs.ErrAdFormatNotFound.
func (r *AdFormatRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.AdFormat, error) {
	const fc = "adFormatRepo_GetByID"

	query := `SELECT ` + adFormatColumns + ` FROM ad_formats WHERE id = $1`

	format, err := scanAdFormat(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrAdFormatNotFound))
	}

	return format, nil
}

// Create inserts the card after the last one and sets its Position.
// Concurrent creates racing for one position fail with errs.ErrConflict.
func (r *AdFormatRepo) Create(ctx context.Context, format *models.AdFormat) error {
	const fc = "adFormatRepo_Create"

	query := `INSERT INTO ad_formats (id, title, description, image_url, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, (SELECT COALESCE(MAX(position), 0) + 1 FROM ad_formats), $5, $6)
		RETURNING position`

	err := r.pool.QueryRow(ctx, query,
		format.ID, format.Title, format.Description, format.ImageURL, format.CreatedAt, format.UpdatedAt,
	).Scan(&format.Position)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrAdFormatNotFound))
	}

	return nil
}

// Update saves title, description, image URL and updated_at; missing id is errs.ErrAdFormatNotFound.
func (r *AdFormatRepo) Update(ctx context.Context, format *models.AdFormat) error {
	const fc = "adFormatRepo_Update"

	query := `UPDATE ad_formats SET title = $2, description = $3, image_url = $4, updated_at = $5 WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, format.ID, format.Title, format.Description, format.ImageURL, format.UpdatedAt)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrAdFormatNotFound))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fc, errs.ErrAdFormatNotFound)
	}

	return nil
}

// Delete removes the card; missing id is errs.ErrAdFormatNotFound.
func (r *AdFormatRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const fc = "adFormatRepo_Delete"

	tag, err := r.pool.Exec(ctx, `DELETE FROM ad_formats WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fc, errs.ErrAdFormatNotFound)
	}

	return nil
}

// Move swaps the card with its neighbour above (up) or below in one transaction.
// No neighbour is a no-op; missing id is errs.ErrAdFormatNotFound. Moves are
// serialized by a table lock (tiny, admin-only table), so concurrent moves never
// deadlock or swap with a stale neighbour; the deferred unique position is checked at commit.
func (r *AdFormatRepo) Move(ctx context.Context, id uuid.UUID, up bool) error {
	const fc = "adFormatRepo_Move"

	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return swapWithNeighbour(ctx, tx, adFormatsTable, id, up)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrAdFormatNotFound))
	}

	return nil
}

// scanAdFormat reads one ad_formats row selected with adFormatColumns.
func scanAdFormat(row scanner) (*models.AdFormat, error) {
	var format models.AdFormat
	err := row.Scan(
		&format.ID,
		&format.Title,
		&format.Description,
		&format.ImageURL,
		&format.Position,
		&format.CreatedAt,
		&format.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &format, nil
}

// swapWithNeighbour locks table and exchanges positions of the row and its neighbour.
// table is always a package constant (never user input) of a table with id and a
// deferred unique position.
func swapWithNeighbour(ctx context.Context, tx pgx.Tx, table string, id uuid.UUID, up bool) error {
	if _, err := tx.Exec(ctx, `LOCK TABLE `+table+` IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}

	var position int
	if err := tx.QueryRow(ctx, `SELECT position FROM `+table+` WHERE id = $1`, id).Scan(&position); err != nil {
		return err
	}

	neighbourQuery := `SELECT id, position FROM ` + table + ` WHERE position > $1 ORDER BY position ASC LIMIT 1`
	if up {
		neighbourQuery = `SELECT id, position FROM ` + table + ` WHERE position < $1 ORDER BY position DESC LIMIT 1`
	}

	var (
		neighbourID       uuid.UUID
		neighbourPosition int
	)
	err := tx.QueryRow(ctx, neighbourQuery, position).Scan(&neighbourID, &neighbourPosition)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE `+table+` SET position = CASE id WHEN $1 THEN $2::int ELSE $3::int END, updated_at = now()
		WHERE id IN ($1, $4)`,
		id, neighbourPosition, position, neighbourID,
	)

	return err
}
