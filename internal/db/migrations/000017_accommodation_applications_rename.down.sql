-- Reverse the data copies from the .up.sql. Each statement back-populates
-- the legacy column from the new one when the legacy column is at its zero
-- value, mirroring the up direction.
--
-- Lossy by design for the bool→timestamp case: applied_at can be derived
-- back into the boolean (presence ⇒ true), but the timestamp's chosen seed
-- (created_at / now()) is lost. That's acceptable — Wave 2b is additive,
-- and Wave 2c's drop migration is what removes the legacy columns for good.

BEGIN;

-- Reverse 6: adjusted_time_limit → extended_time_limit.
UPDATE accommodation_applications
SET extended_time_limit = adjusted_time_limit
WHERE extended_time_limit IS NULL
  AND adjusted_time_limit IS NOT NULL;

-- Reverse 5: adjusted_due_at → extended_due_at.
UPDATE accommodation_applications
SET extended_due_at = adjusted_due_at
WHERE extended_due_at IS NULL
  AND adjusted_due_at IS NOT NULL;

-- Reverse 4: applied_at → applied (truthiness only).
UPDATE accommodation_applications
SET applied = true
WHERE applied = false
  AND applied_at IS NOT NULL;

-- Reverse 3: (resource_type='quiz', resource_id) → quiz_id.
UPDATE accommodation_applications
SET quiz_id = resource_id
WHERE quiz_id IS NULL
  AND resource_type = 'quiz'
  AND resource_id > 0;

-- Reverse 2: (resource_type='assignment', resource_id) → assignment_id.
UPDATE accommodation_applications
SET assignment_id = resource_id
WHERE assignment_id IS NULL
  AND resource_type = 'assignment'
  AND resource_id > 0;

-- Reverse 1: accommodation_id → student_accommodation_id.
--    student_accommodation_id is NOT NULL in the original schema; on a
--    freshly-migrated DB this is a no-op, on a partially-migrated one it
--    re-fills the legacy column.
UPDATE accommodation_applications
SET student_accommodation_id = accommodation_id
WHERE (student_accommodation_id IS NULL OR student_accommodation_id = 0)
  AND accommodation_id > 0;

COMMIT;
