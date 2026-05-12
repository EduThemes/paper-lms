-- Wave 2c: drop legacy columns on accommodation_applications. DATA-DESTRUCTIVE.
--
-- Deprecation window: added 2026-05-11. Operators with production data that
-- predates this migration MUST have run 000017 first; that migration copies
-- legacy data into the new GORM-model columns. Once this migration runs the
-- dropped columns are gone — the .down.sql can recreate the shape but not
-- the data.
--
-- The eight columns dropped here fall into four categories:
--
--   1. Wave 2b RENAME / POLYMORPHIC sources (data now lives in their new
--      target):
--        student_accommodation_id → accommodation_id
--        assignment_id            → (resource_type='assignment', resource_id)
--        quiz_id                  → (resource_type='quiz',       resource_id)
--        applied (bool)           → applied_at (timestamp)
--        extended_due_at          → adjusted_due_at
--        extended_time_limit      → adjusted_time_limit
--
--   2. SOFT_DELETE_LEFTOVER: deleted_at — the model removed gorm.DeletedAt.
--
--   3. Intentionally-removed audit columns: updated_at — the model is
--      append-only; updates aren't tracked, so this column was always going
--      to lag and is safe to drop.

BEGIN;

-- Wave 2b sources
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS applied;
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS assignment_id;
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS quiz_id;
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS student_accommodation_id;
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS extended_due_at;
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS extended_time_limit;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS deleted_at;

-- Intentional model omission (append-only audit table)
ALTER TABLE accommodation_applications DROP COLUMN IF EXISTS updated_at;

COMMIT;
