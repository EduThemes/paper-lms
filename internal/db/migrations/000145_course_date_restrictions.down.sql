-- 000145_course_date_restrictions.down.sql

BEGIN;

ALTER TABLE courses
    DROP COLUMN IF EXISTS restrict_student_past_view,
    DROP COLUMN IF EXISTS restrict_student_future_view,
    DROP COLUMN IF EXISTS restrict_enrollments_to_course_dates;

COMMIT;
