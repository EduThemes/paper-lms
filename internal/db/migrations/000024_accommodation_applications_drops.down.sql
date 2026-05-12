-- Reverse of 000024: re-add the dropped columns with their original types
-- and defaults from migration 000001. Structural only — the row data is LOST.
--
-- IMPORTANT: rows that existed when 000024 ran no longer have legacy-column
-- values. After this down runs, all legacy columns are NULL/default. If you
-- need the legacy data back, you must restore from a backup taken before
-- 000024 ran; the .up.sql is data-destructive by design.
--
-- The original 000001 schema declared student_accommodation_id as NOT NULL
-- without a default. Recreating that constraint here would fail on populated
-- tables (rows would have NULL in the new column), so we re-add it as
-- nullable. Operators rolling back are presumably also restoring data and
-- can re-tighten the constraint separately if they need it.

BEGIN;

ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS extended_time_limit bigint;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS extended_due_at timestamptz;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS student_accommodation_id bigint;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS quiz_id bigint;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS assignment_id bigint;
ALTER TABLE accommodation_applications ADD COLUMN IF NOT EXISTS applied boolean DEFAULT false;

COMMIT;
