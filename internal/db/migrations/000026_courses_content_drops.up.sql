-- Wave 2c: drop legacy columns on courses + content domain tables. DATA-DESTRUCTIVE.
--
-- Deprecation window: added 2026-05-11. Operators with production data that
-- predates this migration MUST have run 000019 first; that migration copies
-- legacy data into the new GORM-model columns. Once this migration runs the
-- dropped columns are gone — the .down.sql can recreate the shape but not
-- the data.
--
-- Tables covered: courses, course_sections, enrollments, enrollment_terms,
--   course_paces, course_pace_module_items, course_home_buttons,
--   todays_lesson_overrides, course_visits, attendance_records,
--   context_modules, content_tags, module_prerequisites, wiki_pages,
--   folders, attachments, document_annotations.
--
-- Columns dropped fall into these categories:
--
--   1. Wave 2b RENAME sources (data now lives in the new target column):
--        courses.apply_assignment_group_weights → apply_group_weights
--        courses.home_page_type                 → default_view
--        attendance_records.recorded_by         → marked_by_id
--        course_visits.last_page_url            → last_url
--        document_annotations.page             → page_number
--        document_annotations.parent_id        → parent_annotation_id
--        document_annotations.resolved_by      → resolved_by_user_id
--        document_annotations.stroke_data      → path_data
--        document_annotations.resolved (bool)  → resolved_at (timestamp)
--        todays_lesson_overrides.module_id     → link_id (+ link_type)
--
--   2. SOFT_DELETE_LEFTOVER: deleted_at — gorm.DeletedAt removed from models.
--
--   3. UNKNOWN with zero non-test Go references (safe to drop):
--        courses: description, grading_standard_enabled, grading_standard_id,
--                 storage_quota
--        course_home_buttons: created_at, updated_at, link_target
--        course_visits: created_at, last_module_id, last_module_item_id,
--                       last_visited_at
--        enrollments: enrollment_type, limit_privileges_to_course_section,
--                     sis_import_id
--        context_modules: prerequisite_module_ids
--        wiki_pages: published, lock_at, unlock_at
--        folders: hidden, lock_at, locked, unlock_at
--        attachments: hidden, lock_at, locked, unlock_at, url
--        document_annotations: attachment_id
--        todays_lesson_overrides: created_at, updated_at
--
--   4. KEPT — column has active non-test Go references as a DB column:
--        courses.enrollment_term_id — used in hand-written SQL WHERE clause at
--          internal/service/enrollment_term_service.go:73; no matching GORM
--          field but the column is actively queried, so it is retained here.

BEGIN;

-- ============ courses ============

-- Wave 2b sources
ALTER TABLE courses DROP COLUMN IF EXISTS apply_assignment_group_weights;
ALTER TABLE courses DROP COLUMN IF EXISTS home_page_type;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE courses DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs
ALTER TABLE courses DROP COLUMN IF EXISTS description;
ALTER TABLE courses DROP COLUMN IF EXISTS grading_standard_enabled;
ALTER TABLE courses DROP COLUMN IF EXISTS grading_standard_id;
ALTER TABLE courses DROP COLUMN IF EXISTS storage_quota;

-- ============ course_sections ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE course_sections DROP COLUMN IF EXISTS deleted_at;

-- ============ enrollments ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE enrollments DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (enrollment.Type GORM field maps to "type", not "enrollment_type")
ALTER TABLE enrollments DROP COLUMN IF EXISTS enrollment_type;
ALTER TABLE enrollments DROP COLUMN IF EXISTS limit_privileges_to_course_section;
ALTER TABLE enrollments DROP COLUMN IF EXISTS sis_import_id;

-- ============ enrollment_terms ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE enrollment_terms DROP COLUMN IF EXISTS deleted_at;

-- ============ course_paces ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE course_paces DROP COLUMN IF EXISTS deleted_at;

-- ============ course_pace_module_items ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE course_pace_module_items DROP COLUMN IF EXISTS deleted_at;

-- ============ course_home_buttons ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE course_home_buttons DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (CourseHomeButton model has no timestamp or link_target fields)
ALTER TABLE course_home_buttons DROP COLUMN IF EXISTS created_at;
ALTER TABLE course_home_buttons DROP COLUMN IF EXISTS updated_at;
ALTER TABLE course_home_buttons DROP COLUMN IF EXISTS link_target;

-- ============ todays_lesson_overrides ============

-- Wave 2b source (data copied to link_id + link_type='module')
ALTER TABLE todays_lesson_overrides DROP COLUMN IF EXISTS module_id;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE todays_lesson_overrides DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (TodaysLessonOverride model has no timestamps)
ALTER TABLE todays_lesson_overrides DROP COLUMN IF EXISTS created_at;
ALTER TABLE todays_lesson_overrides DROP COLUMN IF EXISTS updated_at;

-- ============ course_visits ============

-- Wave 2b source (data copied to last_url)
ALTER TABLE course_visits DROP COLUMN IF EXISTS last_page_url;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE course_visits DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (CourseVisit model has no created_at or module tracking)
ALTER TABLE course_visits DROP COLUMN IF EXISTS created_at;
ALTER TABLE course_visits DROP COLUMN IF EXISTS last_module_id;
ALTER TABLE course_visits DROP COLUMN IF EXISTS last_module_item_id;
ALTER TABLE course_visits DROP COLUMN IF EXISTS last_visited_at;

-- ============ attendance_records ============

-- Wave 2b source (data copied to marked_by_id)
ALTER TABLE attendance_records DROP COLUMN IF EXISTS recorded_by;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE attendance_records DROP COLUMN IF EXISTS deleted_at;

-- ============ context_modules ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE context_modules DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (prerequisites now stored in module_prerequisites table)
ALTER TABLE context_modules DROP COLUMN IF EXISTS prerequisite_module_ids;

-- ============ content_tags ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE content_tags DROP COLUMN IF EXISTS deleted_at;

-- ============ wiki_pages ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (WikiPage model uses workflow_state, not published bool)
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS published;
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS lock_at;
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS unlock_at;

-- ============ folders ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE folders DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (Folder model has no locking or visibility fields)
ALTER TABLE folders DROP COLUMN IF EXISTS hidden;
ALTER TABLE folders DROP COLUMN IF EXISTS lock_at;
ALTER TABLE folders DROP COLUMN IF EXISTS locked;
ALTER TABLE folders DROP COLUMN IF EXISTS unlock_at;

-- ============ attachments ============

-- SOFT_DELETE_LEFTOVER
ALTER TABLE attachments DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (Attachment model has StoragePath, not url; no locking fields)
ALTER TABLE attachments DROP COLUMN IF EXISTS hidden;
ALTER TABLE attachments DROP COLUMN IF EXISTS lock_at;
ALTER TABLE attachments DROP COLUMN IF EXISTS locked;
ALTER TABLE attachments DROP COLUMN IF EXISTS unlock_at;
ALTER TABLE attachments DROP COLUMN IF EXISTS url;

-- ============ document_annotations ============

-- Wave 2b sources
ALTER TABLE document_annotations DROP COLUMN IF EXISTS page;
ALTER TABLE document_annotations DROP COLUMN IF EXISTS parent_id;
ALTER TABLE document_annotations DROP COLUMN IF EXISTS resolved;
ALTER TABLE document_annotations DROP COLUMN IF EXISTS resolved_by;
ALTER TABLE document_annotations DROP COLUMN IF EXISTS stroke_data;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE document_annotations DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / zero non-test refs (DocumentAnnotation model has no attachment_id field)
ALTER TABLE document_annotations DROP COLUMN IF EXISTS attachment_id;

COMMIT;
