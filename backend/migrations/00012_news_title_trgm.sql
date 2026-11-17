-- +goose Up
-- Trigram index for the public title search (title ILIKE '%query%'),
-- which otherwise scans the whole table. pg_trgm is a trusted extension
-- (PostgreSQL 13+), so the database owner may create it.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_news_title_trgm ON news USING gin (title gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS idx_news_title_trgm;
