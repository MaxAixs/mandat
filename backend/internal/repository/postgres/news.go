// Package postgres implements the news repository over PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// newsColumns is the canonical column list shared by SELECT queries.
const newsColumns = `id, slug, category, title, description, text, language,
	status, views, featured, main_event, image_url, source, source_url, published_at, created_at, updated_at`

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
	selectsFlags := news.Featured || news.MainEvent
	if news.Status == models.StatusPublished && selectsFlags {
		err = r.withPublishedFlagsLock(ctx, news.Category, news.Featured, news.MainEvent, func(tx pgx.Tx) error {
			return insertNews(ctx, tx, news)
		})
	} else {
		err = insertNews(ctx, r.pool, news)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrNewsNotFound))
	}

	return nil
}

func insertNews(ctx context.Context, db commandExecutor, news *models.News) error {
	const fc = "newsRepo_insertNews"

	query := `INSERT INTO news
		(id, slug, category, title, description, text, language, status,
		featured, main_event, image_url, source, source_url, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	_, err := db.Exec(ctx, query,
		news.ID, news.Slug, news.Category, news.Title, news.Description,
		news.Text, news.Language, news.Status, news.Featured, news.MainEvent,
		news.ImageUrl, news.Source, news.SourceURL, news.PublishedAt, news.CreatedAt, news.UpdatedAt,
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
		return nil, fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrNewsNotFound))
	}

	return news, nil
}

// GetByID returns one article or errs.ErrNewsNotFound.
func (r *NewsRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.News, error) {
	const fc = "newsRepo_GetByID"

	query := `SELECT ` + newsColumns + ` FROM news WHERE id = $1`

	news, err := scanNews(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrNewsNotFound))
	}

	return news, nil
}

// List returns a page of articles (newest first).
func (r *NewsRepo) List(ctx context.Context, filter models.ListFilter) ([]*models.News, error) {
	const fc = "newsRepo_List"

	listQuery := `SELECT ` + newsColumns + ` FROM news`

	var (
		args       []any
		conditions []string
	)
	if filter.Status != nil {
		args = append(args, *filter.Status)
		conditions = append(conditions, fmt.Sprintf(`status = $%d`, len(args)))
	}
	if filter.Category != nil {
		args = append(args, *filter.Category)
		conditions = append(conditions, fmt.Sprintf(`category = $%d`, len(args)))
	}
	if len(conditions) > 0 {
		listQuery += ` WHERE ` + strings.Join(conditions, ` AND `)
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	listQuery += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, limitPos, offsetPos)
	args = append(args, filter.Limit, (filter.Page-1)*filter.Limit)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrNewsNotFound))
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

	query := `SELECT ` + newsColumns + ` FROM news WHERE status = $1`
	args := []any{models.StatusPublished}
	if filter.Category != nil {
		args = append(args, *filter.Category)
		query += fmt.Sprintf(` AND category = $%d`, len(args))
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	query += fmt.Sprintf(
		` ORDER BY main_event DESC, featured DESC, published_at DESC NULLS LAST, id DESC LIMIT $%d OFFSET $%d`,
		limitPos,
		offsetPos,
	)
	args = append(args, filter.Limit, (filter.Page-1)*filter.Limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrNewsNotFound))
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

// SearchPublished returns up to limit published articles whose title contains query.
// Title matching relies on ILIKE case folding, which follows the database
// collation: the database must use a UTF-8 collation for Cyrillic search
// to stay case-insensitive.
func (r *NewsRepo) SearchPublished(ctx context.Context, query string, limit int) ([]*models.News, error) {
	const fc = "newsRepo_SearchPublished"

	rows, err := r.pool.Query(
		ctx,
		`SELECT `+newsColumns+` FROM news
		WHERE status = $1 AND title ILIKE '%' || $2 || '%' ESCAPE '\'
		ORDER BY published_at DESC NULLS LAST, id DESC LIMIT $3`,
		models.StatusPublished,
		escapeLike(query),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrNewsNotFound))
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
	selectsFlags := news.Featured || news.MainEvent
	if news.Status == models.StatusPublished && selectsFlags {
		err = r.withPublishedFlagsLock(ctx, news.Category, news.Featured, news.MainEvent, func(tx pgx.Tx) error {
			return updateNews(ctx, tx, news)
		})
	} else {
		err = updateNews(ctx, r.pool, news)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", fc, mapError(err, errs.ErrNewsNotFound))
	}

	return nil
}

func (r *NewsRepo) withPublishedFlagsLock(
	ctx context.Context,
	category models.Category,
	featured bool,
	mainEvent bool,
	write func(pgx.Tx) error,
) (err error) {
	const fc = "newsRepo_withPublishedFlagsLock"

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
	if featured {
		if _, err = tx.Exec(ctx, `UPDATE news SET featured = FALSE
			WHERE status = 'published' AND featured = TRUE`); err != nil {
			return fmt.Errorf("%s: clear featured: %w", fc, err)
		}
	}
	if mainEvent {
		if _, err = tx.Exec(ctx, `UPDATE news SET main_event = FALSE
			WHERE status = 'published' AND main_event = TRUE AND category = $1`, category); err != nil {
			return fmt.Errorf("%s: clear main event: %w", fc, err)
		}
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
		&news.MainEvent, &news.ImageUrl, &news.Source, &news.SourceURL, &news.PublishedAt, &news.CreatedAt, &news.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &news, nil
}

func updateNews(ctx context.Context, db commandExecutor, news *models.News) error {
	const fc = "newsRepo_updateNews"

	query := `UPDATE news SET
		slug = $2, category = $3, title = $4, description = $5, text = $6,
		language = $7, status = $8, featured = $9, main_event = $10, image_url = $11,
		source = $12, source_url = $13, published_at = $14, updated_at = $15
		WHERE id = $1`

	tag, err := db.Exec(ctx, query,
		news.ID, news.Slug, news.Category, news.Title, news.Description, news.Text,
		news.Language, news.Status, news.Featured, news.MainEvent, news.ImageUrl,
		news.Source, news.SourceURL, news.PublishedAt, news.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", fc, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fc, errs.ErrNewsNotFound)
	}

	return nil
}

// escapeLike escapes the LIKE pattern metacharacters in s.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
