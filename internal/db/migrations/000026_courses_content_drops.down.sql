-- Reverse of 000026: re-add the dropped columns with their original types
-- and defaults from migration 000001. Structural only — the row data is LOST.
--
-- IMPORTANT: rows that existed when 000026 ran no longer have legacy-column
-- values. After this down runs, all restored columns are NULL/default. If you
-- need the legacy data back, you must restore from a backup taken before
-- 000026 ran; the .up.sql is data-destructive by design.
--
-- NOT NULL constraints from 000001 (e.g. todays_lesson_overrides.module_id,
-- attendance_records.recorded_by) are recreated as nullable here. Operators
-- rolling back are presumably also restoring data and can re-tighten
-- constraints separately if needed.

BEGIN;

-- ============ document_annotations ============

ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS attachment_id bigint;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS stroke_data text;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS resolved_by bigint;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS resolved boolean DEFAULT false;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS parent_id bigint;
ALTER TABLE document_annotations ADD COLUMN IF NOT EXISTS page bigint DEFAULT 1;

-- ============ attachments ============

ALTER TABLE attachments ADD COLUMN IF NOT EXISTS url text;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS unlock_at timestamptz;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS locked boolean DEFAULT false;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS lock_at timestamptz;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS hidden boolean DEFAULT false;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ folders ============

ALTER TABLE folders ADD COLUMN IF NOT EXISTS unlock_at timestamptz;
ALTER TABLE folders ADD COLUMN IF NOT EXISTS locked boolean DEFAULT false;
ALTER TABLE folders ADD COLUMN IF NOT EXISTS lock_at timestamptz;
ALTER TABLE folders ADD COLUMN IF NOT EXISTS hidden boolean DEFAULT false;
ALTER TABLE folders ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ wiki_pages ============

ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS unlock_at timestamptz;
ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS lock_at timestamptz;
ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS published boolean DEFAULT false;
ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ content_tags ============

ALTER TABLE content_tags ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ context_modules ============

ALTER TABLE context_modules ADD COLUMN IF NOT EXISTS prerequisite_module_ids text;
ALTER TABLE context_modules ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ attendance_records ============

ALTER TABLE attendance_records ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE attendance_records ADD COLUMN IF NOT EXISTS recorded_by bigint;

-- ============ course_visits ============

ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS last_visited_at timestamptz;
ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS last_module_item_id bigint;
ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS last_module_id bigint;
ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE course_visits ADD COLUMN IF NOT EXISTS last_page_url text;

-- ============ todays_lesson_overrides ============

ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
-- originally NOT NULL; restored as nullable (data loss — see header note)
ALTER TABLE todays_lesson_overrides ADD COLUMN IF NOT EXISTS module_id bigint;

-- ============ course_home_buttons ============

ALTER TABLE course_home_buttons ADD COLUMN IF NOT EXISTS link_target text;
ALTER TABLE course_home_buttons ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE course_home_buttons ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE course_home_buttons ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ course_pace_module_items ============

ALTER TABLE course_pace_module_items ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ course_paces ============

ALTER TABLE course_paces ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ enrollment_terms ============

ALTER TABLE enrollment_terms ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ enrollments ============

ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS sis_import_id bigint;
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS limit_privileges_to_course_section boolean DEFAULT false;
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS enrollment_type text;
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ course_sections ============

ALTER TABLE course_sections ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============ courses ============

ALTER TABLE courses ADD COLUMN IF NOT EXISTS storage_quota bigint;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS grading_standard_id bigint;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS grading_standard_enabled boolean DEFAULT false;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS description text;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS home_page_type text DEFAULT 'modules';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS apply_assignment_group_weights boolean DEFAULT false;

COMMIT;
