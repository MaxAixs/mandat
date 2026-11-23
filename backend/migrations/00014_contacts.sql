-- +goose Up
-- Contacts of the /contacts page, managed from the admin panel.
-- The type picks the icon and the link; position is unique but deferred, so two
-- contacts can swap places in one transaction.
CREATE TABLE contacts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type       TEXT NOT NULL CHECK (type IN ('email', 'telegram', 'instagram', 'threads', 'facebook', 'phone', 'website')),
    value      TEXT NOT NULL CHECK (char_length(value) BETWEEN 1 AND 254),
    label      TEXT NOT NULL DEFAULT '' CHECK (char_length(label) <= 40),
    position   INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT contacts_position_key UNIQUE (position) DEFERRABLE INITIALLY DEFERRED
);

-- The contacts that were hardcoded in frontend/src/constants/contacts.ts.
INSERT INTO contacts (type, value, position) VALUES
    ('email', 'mandat@gmail.com', 1),
    ('telegram', 'mandat', 2);

-- +goose Down
DROP TABLE IF EXISTS contacts;
