-- +goose Up
-- Rubric names changed on 2026-10-28 to match the site navigation:
--   Аналітика     -> Міжнародні новини
--   Інсайди       -> Спорт
--   Важливо       -> Інтерв'ю
--   Розслідування -> Про нас
ALTER TABLE news DROP CONSTRAINT IF EXISTS news_category_check;

UPDATE news
SET category = CASE category
    WHEN 'Аналітика' THEN 'Міжнародні новини'
    WHEN 'Інсайди' THEN 'Спорт'
    WHEN 'Важливо' THEN 'Інтерв''ю'
    WHEN 'Розслідування' THEN 'Про нас'
    ELSE category
END
WHERE category IN ('Аналітика', 'Інсайди', 'Важливо', 'Розслідування');

ALTER TABLE news
    ADD CONSTRAINT news_category_check
    CHECK (category IN ('Новини', 'Міжнародні новини', 'Спорт', 'Інтерв''ю', 'Про нас'));

-- +goose Down
ALTER TABLE news DROP CONSTRAINT IF EXISTS news_category_check;

UPDATE news
SET category = CASE category
    WHEN 'Міжнародні новини' THEN 'Аналітика'
    WHEN 'Спорт' THEN 'Інсайди'
    WHEN 'Інтерв''ю' THEN 'Важливо'
    WHEN 'Про нас' THEN 'Розслідування'
    ELSE category
END
WHERE category IN ('Міжнародні новини', 'Спорт', 'Інтерв''ю', 'Про нас');

ALTER TABLE news
    ADD CONSTRAINT news_category_check
    CHECK (category IN ('Новини', 'Аналітика', 'Інсайди', 'Важливо', 'Розслідування'));
