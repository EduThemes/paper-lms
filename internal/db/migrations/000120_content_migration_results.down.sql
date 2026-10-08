-- 000120_content_migration_results.down.sql
BEGIN;
ALTER TABLE content_migrations DROP COLUMN IF EXISTS results;
COMMIT;
