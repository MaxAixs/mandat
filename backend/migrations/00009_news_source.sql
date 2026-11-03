-- +goose Up
ALTER TABLE news ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE news ADD COLUMN source_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE news DROP COLUMN source_url;
ALTER TABLE news DROP COLUMN source;
