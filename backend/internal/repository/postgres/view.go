package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ViewDB is the PostgreSQL surface used by ViewRepo.
type ViewDB interface {
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// ViewRepo records throttled public article views.
type ViewRepo struct {
	db ViewDB
}

// NewViewRepo creates a ViewRepo backed by db.
func NewViewRepo(db ViewDB) *ViewRepo {
	return &ViewRepo{db: db}
}

// RecordView increments a published article at most once per viewer cooldown.
func (r *ViewRepo) RecordView(
	ctx context.Context,
	slug string,
	viewerID uuid.UUID,
	cooldown time.Duration,
) (int64, error) {
	const fc = "viewRepo_RecordView"

	query := `WITH target AS (
		SELECT id FROM news WHERE slug = $1 AND status = 'published'
	), accepted AS (
		INSERT INTO news_viewers (news_id, viewer_id, last_viewed_at)
		SELECT id, $2, now() FROM target
		ON CONFLICT (news_id, viewer_id) DO UPDATE
		SET last_viewed_at = now()
		WHERE news_viewers.last_viewed_at <= now() - make_interval(secs => $3)
		RETURNING news_id
	), incremented AS (
		UPDATE news AS n SET views = n.views + 1
		FROM accepted AS a WHERE n.id = a.news_id
		RETURNING n.views
	)
	SELECT COALESCE(
		(SELECT views FROM incremented),
		(SELECT views FROM news WHERE id = (SELECT id FROM target))
	)
	WHERE EXISTS (SELECT 1 FROM target)`

	var views int64
	if err := r.db.QueryRow(ctx, query, slug, viewerID, cooldown.Seconds()).Scan(&views); err != nil {
		return 0, fmt.Errorf("%s: %w", fc, mapError(err))
	}

	return views, nil
}
