// Package postgres implements the news repository over PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// uniqueViolation is the PostgreSQL SQLSTATE for unique constraint violations.
const uniqueViolation = "23505"

// newsColumns is the canonical column list shared by SELECT queries.
const newsColumns = `id, slug, category, title, description, text, language,
	status, views, featured, image_url, published_at, created_at, updated_at`

// NewsRepo persists news articles in PostgreSQL.
type NewsRepo struct {
	pool *pgxpool.Pool
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// NewNewsRepo creates a NewsRepo backed by pool.
func NewNewsRepo(pool *pgxpool.Pool) *NewsRepo {
	return &NewsRepo{pool: pool}
}

// Create inserts a news article with caller-assigned ID and timestamps.
func (r *NewsRepo) Create(ctx context.Context, news *models.News) error {
	const fc = "newsRepo_Create"

	var err error
	if news.Status == models.StatusPublished && news.Featured {
		err = r.withPublishedFeaturedLock(ctx, func(tx pgx.Tx) error {
			return insertNews(ctx, tx, news)
		})
	} else {
		err = insertNews(ctx, r.pool, news)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err))
	}

	return nil
}

func insertNews(ctx context.Context, db commandExecutor, news *models.News) error {
	const fc = "newsRepo_insertNews"

	query := `INSERT INTO news
		(id, slug, category, title, description, text, language, status,
		featured, image_url, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := db.Exec(ctx, query,
		news.ID, news.Slug, news.Category, news.Title, news.Description,
		news.Text, news.Language, news.Status, news.Featured, news.ImageUrl,
		news.PublishedAt, news.CreatedAt, news.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}

	return nil
}

// GetBySlug returns one article or errs.ErrNewsNotFound.
func (r *NewsRepo) GetBySlug(ctx context.Context, slug string) (*models.News, error) {
	const fc = "newsRepo_GetBySlug"

	query := `SELECT ` + newsColumns + ` FROM news WHERE slug = $1`

	news, err := scanNews(r.pool.QueryRow(ctx, query, slug))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err))
	}

	return news, nil
}

// GetByID returns one article or errs.ErrNewsNotFound.
func (r *NewsRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.News, error) {
	const fc = "newsRepo_GetByID"

	query := `SELECT ` + newsColumns + ` FROM news WHERE id = $1`

	news, err := scanNews(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err))
	}

	return news, nil
}

// List returns a page of articles (newest first).
func (r *NewsRepo) List(ctx context.Context, filter models.ListFilter) ([]*models.News, error) {
	const fc = "newsRepo_List"

	listQuery := `SELECT ` + newsColumns + ` FROM news`

	var args []any
	if filter.Status != nil {
		listQuery += ` WHERE status = $1`
		args = append(args, *filter.Status)
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	listQuery += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, limitPos, offsetPos)
	args = append(args, filter.Limit, (filter.Page-1)*filter.Limit)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err))
	}
	defer rows.Close()

	var items []*models.News
	for rows.Next() {
		news, err := scanNews(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", fc, err)
		}
		items = append(items, news)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", fc, err)
	}

	return items, nil
}

// ListPublished returns a page of published articles, newest publication first.
func (r *NewsRepo) ListPublished(ctx context.Context, filter models.ListFilter) ([]*models.News, error) {
	const fc = "newsRepo_ListPublished"

	query := `SELECT ` + newsColumns + ` FROM news
		WHERE status = $1
		ORDER BY featured DESC, published_at DESC NULLS LAST, id DESC LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query,
		models.StatusPublished,
		filter.Limit,
		(filter.Page-1)*filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err))
	}
	defer rows.Close()

	var items []*models.News
	for rows.Next() {
		news, err := scanNews(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: scan: %w", fc, err)
		}
		items = append(items, news)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", fc, err)
	}

	return items, nil
}

// Update overwrites all editable fields of the article with news.ID.
func (r *NewsRepo) Update(ctx context.Context, news *models.News) error {
	const fc = "newsRepo_Update"

	var err error
	if news.Status == models.StatusPublished && news.Featured {
		err = r.withPublishedFeaturedLock(ctx, func(tx pgx.Tx) error {
			return updateNews(ctx, tx, news)
		})
	} else {
		err = updateNews(ctx, r.pool, news)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err))
	}

	return nil
}

func updateNews(ctx context.Context, db commandExecutor, news *models.News) error {
	const fc = "newsRepo_updateNews"

	query := `UPDATE news SET
		slug = $2, category = $3, title = $4, description = $5, text = $6,
		language = $7, status = $8, featured = $9, image_url = $10,
		published_at = $11, updated_at = $12
		WHERE id = $1`

	tag, err := db.Exec(ctx, query,
		news.ID, news.Slug, news.Category, news.Title, news.Description, news.Text,
		news.Language, news.Status, news.Featured, news.ImageUrl, news.PublishedAt,
		news.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fc, errs.ErrNewsNotFound)
	}

	return nil
}

func (r *NewsRepo) withPublishedFeaturedLock(ctx context.Context, write func(pgx.Tx) error) (err error) {
	const fc = "newsRepo_withPublishedFeaturedLock"

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin: %w", fc, err)
	}
	defer func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("%s: rollback: %w", fc, rollbackErr))
		}
	}()

	if _, err = tx.Exec(ctx, `LOCK TABLE news IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("%s: lock: %w", fc, err)
	}
	if _, err = tx.Exec(ctx, `UPDATE news SET featured = FALSE
		WHERE status = 'published' AND featured = TRUE`); err != nil {
		return fmt.Errorf("%s: clear featured: %w", fc, err)
	}
	if err = write(tx); err != nil {
		return fmt.Errorf("%s: write: %w", fc, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit: %w", fc, err)
	}

	return nil
}

// Delete removes the article with the given id (hard delete).
func (r *NewsRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const fc = "newsRepo_Delete"

	tag, err := r.pool.Exec(ctx, `DELETE FROM news WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fc, errs.ErrNewsNotFound)
	}

	return nil
}

// scanner abstracts QueryRow and Rows for shared scanning logic.
type scanner interface {
	Scan(dest ...any) error
}

func scanNews(row scanner) (*models.News, error) {
	var news models.News

	err := row.Scan(
		&news.ID, &news.Slug, &news.Category, &news.Title, &news.Description,
		&news.Text, &news.Language, &news.Status, &news.Views, &news.Featured,
		&news.ImageUrl, &news.PublishedAt, &news.CreatedAt, &news.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &news, nil
}

// mapError translates PostgreSQL errors into domain sentinel errors.
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNewsNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return fmt.Errorf("%s: %w", pgErr.ConstraintName, errs.ErrConflict)
	}

	return err
}
