-- +goose Up
ALTER TABLE news ADD COLUMN main_event BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX idx_news_single_published_main_event
    ON news (status)
    WHERE status = 'published' AND main_event = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_news_single_published_main_event;
ALTER TABLE news DROP COLUMN main_event;
