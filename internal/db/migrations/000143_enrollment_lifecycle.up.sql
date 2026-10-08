-- 000143_enrollment_lifecycle.up.sql
--
-- Enrollment lifecycle + date-based access + section-limited privileges
-- (admin-enrollments lane, G105 / G106 / G107).
--
--   start_at / end_at   per-enrollment participation window. When BOTH are
--                       set they override section, course and term dates
--                       (Canvas EnrollmentDateBuilder priority).
--   completed_at        stamped when an enrollment is concluded.
--   limit_privileges_to_course_section
--                       a section-limited enrollment only sees (and, for
--                       staff, grades) users in its own section(s).
--
-- limit_privileges_to_course_section was dropped as unused by 000026; it
-- is re-added here with NOT NULL DEFAULT false so the column is a no-op
-- for every existing row.

BEGIN;

ALTER TABLE enrollments
    ADD COLUMN IF NOT EXISTS start_at timestamptz,
    ADD COLUMN IF NOT EXISTS end_at timestamptz,
    ADD COLUMN IF NOT EXISTS completed_at timestamptz,
    ADD COLUMN IF NOT EXISTS limit_privileges_to_course_section boolean NOT NULL DEFAULT false;

COMMIT;
