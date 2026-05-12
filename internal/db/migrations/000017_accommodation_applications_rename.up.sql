-- Wave 2b: data migration for the accommodation_applications domain.
--
-- The original schema (000001) modeled an applied accommodation as a row
-- with a typed FK pair (assignment_id OR quiz_id), a separate
-- student_accommodation_id, and a boolean `applied` flag. The current GORM
-- model collapses those into a polymorphic (resource_type, resource_id)
-- pair, renames student_accommodation_id → accommodation_id, replaces the
-- boolean with an applied_at timestamp, and renames extended_* → adjusted_*.
--
-- Wave 1 backfilled the new columns as NOT NULL without defaults, which
-- means this migration is exercised on populated tables only in scenarios
-- where rows were inserted post-Wave-1 by the application (which writes the
-- new columns directly). The UPDATE guards still matter for any partially-
-- populated state and to make the migration self-documenting.
--
-- All guards use GORM zero-values ('' for text, 0 for bigint, NULL for
-- nullable types). Every statement is idempotent.

BEGIN;

-- 1. student_accommodation_id → accommodation_id (direct rename).
UPDATE accommodation_applications
SET accommodation_id = student_accommodation_id
WHERE accommodation_id = 0
  AND student_accommodation_id IS NOT NULL
  AND student_accommodation_id > 0;

-- 2. assignment_id → (resource_type='assignment', resource_id).
--    Only one of assignment_id / quiz_id was populated in the old schema —
--    the typed FK pair was mutually exclusive.
UPDATE accommodation_applications
SET resource_type = 'assignment',
    resource_id   = assignment_id
WHERE resource_id = 0
  AND assignment_id IS NOT NULL
  AND assignment_id > 0;

-- 3. quiz_id → (resource_type='quiz', resource_id).
UPDATE accommodation_applications
SET resource_type = 'quiz',
    resource_id   = quiz_id
WHERE resource_id = 0
  AND quiz_id IS NOT NULL
  AND quiz_id > 0;

-- 4. applied (bool) → applied_at (timestamptz). The presence of a non-NULL
--    timestamp encodes truthiness. Seed with created_at so the timestamp
--    reflects when the application was originally recorded, falling back to
--    now() if created_at is missing.
UPDATE accommodation_applications
SET applied_at = COALESCE(created_at, now())
WHERE applied = true
  AND applied_at IS NULL;

-- 5. extended_due_at → adjusted_due_at (semantic rename).
UPDATE accommodation_applications
SET adjusted_due_at = extended_due_at
WHERE adjusted_due_at IS NULL
  AND extended_due_at IS NOT NULL;

-- 6. extended_time_limit → adjusted_time_limit (semantic rename).
UPDATE accommodation_applications
SET adjusted_time_limit = extended_time_limit
WHERE adjusted_time_limit IS NULL
  AND extended_time_limit IS NOT NULL;

COMMIT;
