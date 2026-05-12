-- Reverse the data copies from the .up.sql.  Each statement back-populates
-- the legacy column from the new one when the legacy column is at its zero
-- value, mirroring the up direction.
--
-- Lossy by design for the bool→timestamp case: resolved_at can be derived
-- back into the boolean (presence ⇒ true), but the chosen seed timestamp is
-- lost.  That is acceptable — Wave 2b is additive, and Wave 2c's drop
-- migration removes the legacy columns for good.
--
-- For the link_type guard on todays_lesson_overrides: we only reverse rows
-- whose link_type is 'module', since those are the only rows this migration
-- created.

BEGIN;

-- ============ todays_lesson_overrides ============

-- Reverse 5: link_id → module_id (only rows this migration touched have
--   link_type = 'module').
UPDATE todays_lesson_overrides
SET module_id = link_id
WHERE (module_id IS NULL OR module_id = 0)
  AND link_type = 'module'
  AND link_id IS NOT NULL
  AND link_id > 0;

-- ============ document_annotations ============

-- Reverse 4e: resolved_at → resolved (truthiness only).
UPDATE document_annotations
SET resolved = true
WHERE resolved = false
  AND resolved_at IS NOT NULL;

-- Reverse 4d: path_data → stroke_data.
UPDATE document_annotations
SET stroke_data = path_data
WHERE (stroke_data = '' OR stroke_data IS NULL)
  AND path_data IS NOT NULL
  AND path_data <> '';

-- Reverse 4c: resolved_by_user_id → resolved_by.
UPDATE document_annotations
SET resolved_by = resolved_by_user_id
WHERE resolved_by IS NULL
  AND resolved_by_user_id IS NOT NULL;

-- Reverse 4b: parent_annotation_id → parent_id.
UPDATE document_annotations
SET parent_id = parent_annotation_id
WHERE parent_id IS NULL
  AND parent_annotation_id IS NOT NULL;

-- Reverse 4a: page_number → page.
UPDATE document_annotations
SET page = page_number
WHERE (page IS NULL OR page = 0)
  AND page_number IS NOT NULL
  AND page_number > 0;

-- ============ course_visits ============

-- Reverse 3: last_url → last_page_url.
UPDATE course_visits
SET last_page_url = last_url
WHERE (last_page_url = '' OR last_page_url IS NULL)
  AND last_url IS NOT NULL
  AND last_url <> '';

-- ============ attendance_records ============

-- Reverse 2: marked_by_id → recorded_by.
UPDATE attendance_records
SET recorded_by = marked_by_id
WHERE (recorded_by IS NULL OR recorded_by = 0)
  AND marked_by_id IS NOT NULL
  AND marked_by_id > 0;

-- ============ courses ============

-- Reverse 1b: default_view → home_page_type.
UPDATE courses
SET home_page_type = default_view
WHERE (home_page_type = '' OR home_page_type IS NULL OR home_page_type = 'modules')
  AND default_view IS NOT NULL
  AND default_view <> ''
  AND default_view <> 'modules';

-- Reverse 1a: apply_group_weights → apply_assignment_group_weights.
UPDATE courses
SET apply_assignment_group_weights = true
WHERE apply_assignment_group_weights = false
  AND apply_group_weights = true;

COMMIT;
