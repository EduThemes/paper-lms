-- 000061_users_requires_password_reset.down.sql

BEGIN;

ALTER TABLE users
    DROP COLUMN IF EXISTS requires_password_reset;

COMMIT;
