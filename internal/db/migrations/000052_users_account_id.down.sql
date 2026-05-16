-- 000052_users_account_id.down.sql
BEGIN;

DROP INDEX IF EXISTS idx_users_account_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS account_id;

COMMIT;
