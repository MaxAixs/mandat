-- +goose Up
ALTER TABLE news ADD COLUMN video_kind TEXT CHECK (video_kind IN ('file', 'embed'));
ALTER TABLE news ADD COLUMN video_url TEXT NOT NULL DEFAULT '';
ALTER TABLE news ADD CONSTRAINT news_video_consistent CHECK ((video_kind IS NULL) = (video_url = ''));

-- +goose Down
ALTER TABLE news DROP CONSTRAINT news_video_consistent;
ALTER TABLE news DROP COLUMN video_url;
ALTER TABLE news DROP COLUMN video_kind;
