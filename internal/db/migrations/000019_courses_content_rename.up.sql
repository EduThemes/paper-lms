-- Wave 2b: data migration for the courses + content domain.
--
-- The original schema (000001) used several column names that diverged from
-- the current GORM model.  Wave 1 (000016) back-filled the new model columns
-- as ADD COLUMN IF NOT EXISTS, leaving both old and new columns populated
-- by new writes only.  This migration copies any data that was written to the
-- legacy columns before Wave 1 into the canonical GORM columns.
--
-- Tables with COPY work:
--   courses                  — 2 hidden renames
--   attendance_records       — 1 hidden rename
--   course_visits            — 1 hidden rename
--   document_annotations     — 4 hidden renames + 1 bool→timestamp
--   todays_lesson_overrides  — 1 polymorphic refactor
--
-- Tables that are Wave 2b no-ops (all stale cols are SOFT_DELETE_LEFTOVER
-- or UNKNOWN with no matching GORM field):
--   course_sections, enrollments, enrollment_terms, course_paces,
--   course_pace_module_items, course_home_buttons, context_modules,
--   content_tags, module_prerequisites, wiki_pages, folders, attachments
--
-- All guards use GORM zero-values ('' for text, 0 for bigint, NULL for
-- nullable types). Every statement is idempotent.

BEGIN;

-- ============ courses ============

-- 1a. apply_assignment_group_weights → apply_group_weights (hidden rename:
--     GORM snake-cases the field name ApplyGroupWeights, not the json tag).
UPDATE courses
SET apply_group_weights = true
WHERE apply_group_weights = false
  AND apply_assignment_group_weights = true;

-- 1b. home_page_type → default_view (semantic rename: both columns exist in
--     the original schema with the same default 'modules'; home_page_type is
--     the stale one).
UPDATE courses
SET default_view = home_page_type
WHERE (default_view = '' OR default_view = 'modules')
  AND home_page_type IS NOT NULL
  AND home_page_type <> ''
  AND home_page_type <> 'modules';

-- ============ attendance_records ============

-- 2. recorded_by → marked_by_id (hidden rename: GORM field is MarkedByID).
UPDATE attendance_records
SET marked_by_id = recorded_by
WHERE marked_by_id = 0
  AND recorded_by IS NOT NULL
  AND recorded_by > 0;

-- ============ course_visits ============

-- 3. last_page_url → last_url (hidden rename: GORM field is LastURL).
UPDATE course_visits
SET last_url = last_page_url
WHERE (last_url = '' OR last_url IS NULL)
  AND last_page_url IS NOT NULL
  AND last_page_url <> '';

-- ============ document_annotations ============

-- 4a. page → page_number (hidden rename: GORM field is PageNumber).
UPDATE document_annotations
SET page_number = page
WHERE page_number = 0
  AND page IS NOT NULL
  AND page > 0;

-- 4b. parent_id → parent_annotation_id (hidden rename: GORM field is
--     ParentAnnotationID).
UPDATE document_annotations
SET parent_annotation_id = parent_id
WHERE parent_annotation_id IS NULL
  AND parent_id IS NOT NULL;

-- 4c. resolved_by → resolved_by_user_id (hidden rename: GORM field is
--     ResolvedByUserID).
UPDATE document_annotations
SET resolved_by_user_id = resolved_by
WHERE resolved_by_user_id IS NULL
  AND resolved_by IS NOT NULL;

-- 4d. stroke_data → path_data (semantic rename: freehand annotation path
--     data was renamed from stroke_data to path_data to match SVG terminology
--     used in the GORM model).
UPDATE document_annotations
SET path_data = stroke_data
WHERE (path_data = '' OR path_data IS NULL)
  AND stroke_data IS NOT NULL
  AND stroke_data <> '';

-- 4e. resolved (bool) → resolved_at (timestamptz).  Presence of a non-NULL
--     timestamp encodes truthiness.  Seed with updated_at (the most recent
--     write timestamp), falling back to created_at, then now().
UPDATE document_annotations
SET resolved_at = COALESCE(updated_at, created_at, now())
WHERE resolved = true
  AND resolved_at IS NULL;

-- ============ todays_lesson_overrides ============

-- 5. module_id → link_id (polymorphic refactor: the old schema stored only
--    a module reference; the new model generalises to (link_type, link_id).
--    All pre-Wave-1 rows were module links, so set link_type = 'module').
UPDATE todays_lesson_overrides
SET link_id   = module_id,
    link_type = 'module'
WHERE link_id IS NULL
  AND module_id IS NOT NULL
  AND module_id > 0;

COMMIT;
