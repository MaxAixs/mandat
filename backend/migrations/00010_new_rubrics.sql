-- +goose Up
-- Two new rubrics matching the site navigation: Політика, Регіональні новини.
ALTER TABLE news DROP CONSTRAINT IF EXISTS news_category_check;

ALTER TABLE news
    ADD CONSTRAINT news_category_check
    CHECK (category IN ('Новини', 'Міжнародні новини', 'Спорт', 'Інтерв''ю', 'Про нас', 'Політика', 'Регіональні новини'));

-- +goose Down
-- NOTE: downgrading requires re-categorising every row that uses the new
-- rubrics first; otherwise ADD CONSTRAINT fails on existing 'Політика' /
-- 'Регіональні новини' rows and goose rolls the whole migration back.
ALTER TABLE news DROP CONSTRAINT IF EXISTS news_category_check;

ALTER TABLE news
    ADD CONSTRAINT news_category_check
    CHECK (category IN ('Новини', 'Міжнародні новини', 'Спорт', 'Інтерв''ю', 'Про нас'));
