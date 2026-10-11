-- 000111_course_section_crosslisting.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_course_sections_nonxlist_course_id;

ALTER TABLE course_sections
    DROP COLUMN IF EXISTS restrict_enrollments_to_section_dates,
    DROP COLUMN IF EXISTS nonxlist_course_id;

COMMIT;
