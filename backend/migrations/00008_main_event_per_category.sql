-- +goose Up
DROP INDEX IF EXISTS idx_news_single_published_main_event;
CREATE UNIQUE INDEX idx_news_single_published_main_event_per_category
    ON news (category)
    WHERE status = 'published' AND main_event = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_news_single_published_main_event_per_category;
CREATE UNIQUE INDEX idx_news_single_published_main_event
    ON news (status)
    WHERE status = 'published' AND main_event = TRUE;
