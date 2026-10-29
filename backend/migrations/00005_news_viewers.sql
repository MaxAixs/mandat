-- +goose Up
CREATE TABLE news_viewers (
    news_id       UUID NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    viewer_id     UUID NOT NULL,
    last_viewed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (news_id, viewer_id)
);

CREATE INDEX idx_news_viewers_last_viewed_at ON news_viewers (last_viewed_at);

-- +goose Down
DROP TABLE IF EXISTS news_viewers;
