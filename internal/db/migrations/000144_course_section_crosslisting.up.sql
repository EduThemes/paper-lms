-- 000144_course_section_crosslisting.up.sql
--
-- Section management + cross-listing (admin-enrollments lane, G108/G109).
--
--   nonxlist_course_id  the course a cross-listed section originally
--                       belonged to (NULL when the section is not
--                       cross-listed). De-cross-listing moves the section
--                       and its enrollments back there.
--   restrict_enrollments_to_section_dates
--                       when true the section's start_at / end_at bound
--                       participation for its enrollments (Canvas "Users
--                       can only participate between these dates").

BEGIN;

ALTER TABLE course_sections
    ADD COLUMN IF NOT EXISTS nonxlist_course_id bigint,
    ADD COLUMN IF NOT EXISTS restrict_enrollments_to_section_dates boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_course_sections_nonxlist_course_id
    ON course_sections (nonxlist_course_id);

COMMIT;
