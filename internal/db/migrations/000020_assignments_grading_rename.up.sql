-- Wave 2b: data migration for the assignments + grading domain.
--
-- The original schema (000001) modelled several assignment and grading-period
-- columns with names that differ from the current GORM model. Wave 1
-- (000016) added the new GORM-model columns. This migration copies data
-- from the legacy columns into the new ones.
--
-- Tables examined in this domain and their disposition:
--
--   assignments                     → 1 COPY   (peer_reviews → peer_reviews_enabled)
--   assignment_groups               → DEFER     (deleted_at only — soft-delete leftover)
--   assignment_overrides            → DEFER     (deleted_at, set_id, set_type — no target)
--   assignment_override_students    → DEFER     (deleted_at only — soft-delete leftover)
--   submissions                     → DEFER     (deleted_at only — soft-delete leftover)
--   submission_comments             → DEFER     (deleted_at, group_comment_id, hidden — no target)
--   grading_standards               → DEFER     (deleted_at only — soft-delete leftover)
--   grading_period_groups           → 1 COPY   (display_totals_for_all_grading_periods → display_totals)
--                                               (course_id — no target; DEFER)
--   grading_periods                 → DEFER     (deleted_at only — soft-delete leftover)
--   late_policies                   → DEFER     (deleted_at only — soft-delete leftover)
--   rubrics                         → DEFER     (deleted_at only — soft-delete leftover)
--   rubric_associations             → DEFER     (deleted_at only — soft-delete leftover)
--   rubric_assessments              → DEFER     (deleted_at, artifact_id, artifact_type, comments — no target)
--   peer_reviews                    → no stale columns
--   custom_gradebook_columns        → no stale columns
--   custom_gradebook_column_data    → no stale columns
--   comment_bank_items              → no stale columns
--
-- Re-classified UNKNOWNs:
--   assignments.peer_reviews (UNKNOWN → RENAME_CANDIDATE): the GORM model
--     renamed this field to PeerReviewsEnabled (column: peer_reviews_enabled).
--   grading_period_groups.display_totals_for_all_grading_periods (UNKNOWN →
--     RENAME_CANDIDATE): the GORM model shortened the Go field to DisplayTotals
--     (column: display_totals). Wave 1 added the new column.
--
-- All guards use GORM zero-values. Every statement is idempotent.

BEGIN;

-- ── assignments ──────────────────────────────────────────────────────────────
--
-- 1. peer_reviews (bool) → peer_reviews_enabled (bool).
--    The GORM model renamed the field; semantics are identical.
--    Guard: copy only when the new column is still at its zero value (false)
--    and the old column was explicitly set to true.
UPDATE assignments
SET peer_reviews_enabled = true
WHERE peer_reviews_enabled = false
  AND peer_reviews = true;

-- ── grading_period_groups ────────────────────────────────────────────────────
--
-- 2. display_totals_for_all_grading_periods (bool) → display_totals (bool).
--    The GORM model shortened the Go field name from the full Canvas name to
--    DisplayTotals; the underlying semantics are identical.
UPDATE grading_period_groups
SET display_totals = true
WHERE display_totals = false
  AND display_totals_for_all_grading_periods = true;

COMMIT;
