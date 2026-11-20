-- +goose Up
-- Ad format cards of the /advertising page, managed from the admin panel.
-- Position is unique but deferred, so two cards can swap places in one transaction.
CREATE TABLE ad_formats (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 80),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 240),
    image_url   TEXT NOT NULL DEFAULT '',
    position    INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ad_formats_position_key UNIQUE (position) DEFERRABLE INITIALLY DEFERRED
);

-- The cards that were hardcoded in frontend/src/constants/advertising.ts.
INSERT INTO ad_formats (title, description, position) VALUES
    ('Спецпроєкти', 'Окрема сторінка з власним дизайном, що глибоко розкриває одну тему разом із брендом.', 1),
    ('Лонгріди', 'Великі тексти, репортажі та розслідування, у яких бренд стає частиною історії.', 2),
    ('Відеоінтеграції', 'Згадка або окремий блок про бренд у наших відео для соцмереж і сайту.', 3),
    ('Подкасти', 'Партнерський випуск або інтеграція в розмову з гостями «Мандату».', 4),
    ('Інтерактивні формати', 'Тести, калькулятори та інші механіки, з якими читач взаємодіє сам.', 5),
    ('Проєкти під клієнта', 'Придумаємо формат з нуля під вашу задачу, аудиторію та бюджет.', 6),
    ('Банерна реклама', 'Розміщення банерів на сайті з гнучким таргетингом за розділами.', 7),
    ('Партнерський матеріал', 'Стаття від редакції на тему бренду з позначкою «Партнерський матеріал».', 8);

-- +goose Down
DROP TABLE IF EXISTS ad_formats;
