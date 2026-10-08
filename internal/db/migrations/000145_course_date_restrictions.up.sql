-- 000145_course_date_restrictions.up.sql
--
-- Course-level date-access settings (admin-enrollments lane, G106).
--
--   restrict_enrollments_to_course_dates
--                       the course's start_at / end_at override the term
--                       dates for its enrollments.
--   restrict_student_future_view
--                       students/observers can't view the course at all
--                       before their enrollment starts (default: they get
--                       read-only access, as in Canvas).
--   restrict_student_past_view
--                       students/observers lose all access once their
--                       enrollment has concluded (default: read-only).
--
-- All three DEFAULT false, so existing courses keep today's behaviour.

BEGIN;

ALTER TABLE courses
    ADD COLUMN IF NOT EXISTS restrict_enrollments_to_course_dates boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS restrict_student_future_view boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS restrict_student_past_view boolean NOT NULL DEFAULT false;

COMMIT;
