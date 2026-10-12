-- 000099_content_migration_results.up.sql
--
-- Course copy / package import results (G067): per-type counts and the
-- non-fatal issues the worker hit, shown in the import history.
--
-- Until now nothing ran queued content migrations, and the API let a
-- caller create a course_copy row for ANY source course and then PUT it
-- to "queued". The new worker sweeps queued rows, so rows queued before
-- it existed (never authorized against their source course) are failed
-- here instead of being run.

BEGIN;

ALTER TABLE content_migrations ADD COLUMN IF NOT EXISTS results jsonb;

UPDATE content_migrations
   SET workflow_state = 'failed',
       error_message = 'This migration was queued before content migrations ran in the background; start it again.',
       finished_at = now(),
       updated_at = now()
 WHERE workflow_state IN ('queued', 'running');

COMMIT;
