-- 000063_users_suspended.down.sql

BEGIN;

ALTER TABLE users
    DROP COLUMN IF EXISTS suspended;

COMMIT;
