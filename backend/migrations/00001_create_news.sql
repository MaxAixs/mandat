-- +goose Up
CREATE TABLE news (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,
    category    TEXT NOT NULL CHECK (category IN ('Новини', 'Аналітика', 'Інсайди', 'Важливо', 'Розслідування')),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    text        TEXT NOT NULL DEFAULT '',
    language    TEXT NOT NULL DEFAULT 'uk',
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    views       BIGINT NOT NULL DEFAULT 0,
    featured    BOOLEAN NOT NULL DEFAULT FALSE,
    image_url   TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_news_status_published_at ON news (status, published_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_news_status_published_at;
DROP TABLE IF EXISTS news;
