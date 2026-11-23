-- +goose Up
ALTER TABLE news ADD COLUMN image_urls TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE news ADD CONSTRAINT news_image_urls_max CHECK (cardinality(image_urls) <= 10);

-- +goose Down
ALTER TABLE news DROP CONSTRAINT news_image_urls_max;
ALTER TABLE news DROP COLUMN image_urls;
