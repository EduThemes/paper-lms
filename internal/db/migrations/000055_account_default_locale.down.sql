-- 000055_account_default_locale.down.sql
BEGIN;
ALTER TABLE accounts DROP COLUMN IF EXISTS default_locale;
COMMIT;
