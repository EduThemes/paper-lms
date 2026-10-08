-- 000120_content_migration_results.up.sql
--
-- Course copy / package import results (G067): per-type counts and the
-- non-fatal issues the worker hit, shown in the import history.

BEGIN;

ALTER TABLE content_migrations ADD COLUMN IF NOT EXISTS results jsonb;

COMMIT;
