-- 000143_enrollment_lifecycle.down.sql

BEGIN;

ALTER TABLE enrollments
    DROP COLUMN IF EXISTS limit_privileges_to_course_section,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS end_at,
    DROP COLUMN IF EXISTS start_at;

COMMIT;
