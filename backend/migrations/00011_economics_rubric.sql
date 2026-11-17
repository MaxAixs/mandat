-- +goose Up
-- Replace the «Про нас» rubric with «Економіка»: the About page is gone and
-- economics is a news rubric now. No rows use the old value (verified before
-- writing this migration), so only the constraint changes.
ALTER TABLE news DROP CONSTRAINT IF EXISTS news_category_check;

ALTER TABLE news
    ADD CONSTRAINT news_category_check
    CHECK (category IN ('Новини', 'Міжнародні новини', 'Спорт', 'Інтерв''ю', 'Економіка', 'Політика', 'Регіональні новини'));

-- +goose Down
-- NOTE: downgrading requires re-categorising every 'Економіка' row first;
-- otherwise ADD CONSTRAINT fails on existing rows and goose rolls back.
ALTER TABLE news DROP CONSTRAINT IF EXISTS news_category_check;

ALTER TABLE news
    ADD CONSTRAINT news_category_check
    CHECK (category IN ('Новини', 'Міжнародні новини', 'Спорт', 'Інтерв''ю', 'Про нас', 'Політика', 'Регіональні новини'));
