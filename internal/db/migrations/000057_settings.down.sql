-- 000057_settings.down.sql
BEGIN;

DROP INDEX IF EXISTS idx_settings_lookup;
DROP TABLE IF EXISTS settings;

COMMIT;
