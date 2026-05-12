-- Wave 2c: drop legacy columns on assignments + grading domain. DATA-DESTRUCTIVE.
--
-- Deprecation window: added 2026-05-11. Operators with production data that
-- predates this migration MUST have run 000020 first; that migration copies
-- legacy data into the new GORM-model columns. Once this migration runs the
-- dropped columns are gone — the .down.sql can recreate the shape but not
-- the data.
--
-- Columns dropped per table:
--
--   assignments (10 cols):
--     Wave 2b RENAME source:
--       peer_reviews → peer_reviews_enabled
--     SOFT_DELETE_LEFTOVER:
--       deleted_at
--     UNKNOWN, zero code refs, not in GORM model:
--       allowed_extensions, automatic_peer_reviews, grade_group_students_individually,
--       muted, omit_from_final_grade, only_visible_to_overrides, post_to_sis,
--       turnitin_enabled
--
--   assignment_groups (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   submissions (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   submission_comments (3 cols):
--     SOFT_DELETE_LEFTOVER: deleted_at
--     UNKNOWN, zero code refs, not in GORM model: group_comment_id, hidden
--
--   grading_standards (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   grading_period_groups (3 cols):
--     Wave 2b RENAME source:
--       display_totals_for_all_grading_periods → display_totals
--     SOFT_DELETE_LEFTOVER: deleted_at
--     UNKNOWN, zero code refs, not in GORM model: course_id
--
--   grading_periods (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   assignment_overrides (3 cols):
--     SOFT_DELETE_LEFTOVER: deleted_at
--     UNKNOWN, zero code refs on this table, not in GORM model: set_id, set_type
--       (codebase refs to set_id are for conditional_release tables, not this table)
--
--   assignment_override_students (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   late_policies (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   rubrics (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   rubric_associations (1 col):
--     SOFT_DELETE_LEFTOVER: deleted_at
--
--   rubric_assessments (4 cols):
--     SOFT_DELETE_LEFTOVER: deleted_at
--     UNKNOWN, zero code refs on this table, not in GORM model:
--       artifact_id, artifact_type, comments
--       (codebase refs to artifact_id/artifact_type are for portfolio_artifacts,
--        not rubric_assessments; comments refs are for submission_comments/peer_reviews)
--
--   peer_reviews               → no stale columns
--   custom_gradebook_columns   → no stale columns
--   custom_gradebook_column_data → no stale columns
--   comment_bank_items         → no stale columns

BEGIN;

-- ── assignments ───────────────────────────────────────────────────────────────

-- Wave 2b rename source
ALTER TABLE assignments DROP COLUMN IF EXISTS peer_reviews;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE assignments DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN, zero non-test code refs
ALTER TABLE assignments DROP COLUMN IF EXISTS allowed_extensions;
ALTER TABLE assignments DROP COLUMN IF EXISTS automatic_peer_reviews;
ALTER TABLE assignments DROP COLUMN IF EXISTS grade_group_students_individually;
ALTER TABLE assignments DROP COLUMN IF EXISTS muted;
ALTER TABLE assignments DROP COLUMN IF EXISTS omit_from_final_grade;
ALTER TABLE assignments DROP COLUMN IF EXISTS only_visible_to_overrides;
ALTER TABLE assignments DROP COLUMN IF EXISTS post_to_sis;
ALTER TABLE assignments DROP COLUMN IF EXISTS turnitin_enabled;

-- ── assignment_groups ─────────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE assignment_groups DROP COLUMN IF EXISTS deleted_at;

-- ── submissions ───────────────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE submissions DROP COLUMN IF EXISTS deleted_at;

-- ── submission_comments ───────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE submission_comments DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN, zero non-test code refs
ALTER TABLE submission_comments DROP COLUMN IF EXISTS group_comment_id;
ALTER TABLE submission_comments DROP COLUMN IF EXISTS hidden;

-- ── grading_standards ─────────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE grading_standards DROP COLUMN IF EXISTS deleted_at;

-- ── grading_period_groups ─────────────────────────────────────────────────────

-- Wave 2b rename source
ALTER TABLE grading_period_groups DROP COLUMN IF EXISTS display_totals_for_all_grading_periods;

-- SOFT_DELETE_LEFTOVER
ALTER TABLE grading_period_groups DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN, zero code refs on this table
ALTER TABLE grading_period_groups DROP COLUMN IF EXISTS course_id;

-- ── grading_periods ───────────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE grading_periods DROP COLUMN IF EXISTS deleted_at;

-- ── assignment_overrides ──────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE assignment_overrides DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN, zero code refs on assignment_overrides table
ALTER TABLE assignment_overrides DROP COLUMN IF EXISTS set_id;
ALTER TABLE assignment_overrides DROP COLUMN IF EXISTS set_type;

-- ── assignment_override_students ──────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE assignment_override_students DROP COLUMN IF EXISTS deleted_at;

-- ── late_policies ─────────────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE late_policies DROP COLUMN IF EXISTS deleted_at;

-- ── rubrics ───────────────────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE rubrics DROP COLUMN IF EXISTS deleted_at;

-- ── rubric_associations ───────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE rubric_associations DROP COLUMN IF EXISTS deleted_at;

-- ── rubric_assessments ────────────────────────────────────────────────────────

-- SOFT_DELETE_LEFTOVER
ALTER TABLE rubric_assessments DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN, zero code refs on rubric_assessments table
-- (artifact_id/artifact_type refs in codebase belong to portfolio_artifacts;
--  comments refs belong to submission_comments and peer_reviews)
ALTER TABLE rubric_assessments DROP COLUMN IF EXISTS artifact_id;
ALTER TABLE rubric_assessments DROP COLUMN IF EXISTS artifact_type;
ALTER TABLE rubric_assessments DROP COLUMN IF EXISTS comments;

COMMIT;
