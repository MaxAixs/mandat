-- +goose Up
WITH ranked_featured AS (
    SELECT id,
           ROW_NUMBER() OVER (
               ORDER BY published_at DESC NULLS LAST, created_at DESC, id DESC
           ) AS position
    FROM news
    WHERE status = 'published' AND featured = TRUE
)
UPDATE news
SET featured = FALSE
WHERE id IN (SELECT id FROM ranked_featured WHERE position > 1);

CREATE UNIQUE INDEX idx_news_single_published_featured
    ON news (status)
    WHERE status = 'published' AND featured = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_news_single_published_featured;
