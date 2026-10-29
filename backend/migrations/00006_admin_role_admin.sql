-- Canonical staff role name is "admin"; align the constraint and existing rows.

-- +goose Up
ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_check;

UPDATE admin_users
SET role = 'admin', updated_at = now()
WHERE role = 'administrator';

ALTER TABLE admin_users
    ADD CONSTRAINT admin_users_role_check CHECK (role IN ('admin', 'moderator'));

-- +goose Down
ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_check;

UPDATE admin_users
SET role = 'administrator', updated_at = now()
WHERE role = 'admin';

ALTER TABLE admin_users
    ADD CONSTRAINT admin_users_role_check CHECK (role IN ('administrator', 'moderator'));
