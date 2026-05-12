-- Reverse of 000027: re-add the dropped columns with their original types
-- and defaults from migrations 000001 and 000016. Structural only — the row
-- data is LOST.
--
-- IMPORTANT: rows that existed when 000027 ran no longer have legacy-column
-- values. After this down runs, all restored columns are NULL/default. If you
-- need the legacy data back, you must restore from a backup taken before
-- 000027 ran; the .up.sql is data-destructive by design.
--
-- Several columns (deleted_at, set_id, set_type, etc.) were originally NOT
-- NULL or had specific constraints in 000001. Recreating those constraints
-- here would fail on populated tables, so we re-add all columns as nullable
-- without constraints. Operators rolling back are presumably also restoring
-- data and can re-tighten constraints separately if needed.

BEGIN;

-- ── rubric_assessments ────────────────────────────────────────────────────────

ALTER TABLE rubric_assessments ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE rubric_assessments ADD COLUMN IF NOT EXISTS artifact_type text;
ALTER TABLE rubric_assessments ADD COLUMN IF NOT EXISTS artifact_id bigint;
ALTER TABLE rubric_assessments ADD COLUMN IF NOT EXISTS comments text;

-- ── rubric_associations ───────────────────────────────────────────────────────

ALTER TABLE rubric_associations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── rubrics ───────────────────────────────────────────────────────────────────

ALTER TABLE rubrics ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── late_policies ─────────────────────────────────────────────────────────────

ALTER TABLE late_policies ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── assignment_override_students ──────────────────────────────────────────────

ALTER TABLE assignment_override_students ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── assignment_overrides ──────────────────────────────────────────────────────

ALTER TABLE assignment_overrides ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE assignment_overrides ADD COLUMN IF NOT EXISTS set_type text;
ALTER TABLE assignment_overrides ADD COLUMN IF NOT EXISTS set_id bigint;

-- ── grading_periods ───────────────────────────────────────────────────────────

ALTER TABLE grading_periods ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── grading_period_groups ─────────────────────────────────────────────────────

ALTER TABLE grading_period_groups ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE grading_period_groups ADD COLUMN IF NOT EXISTS course_id bigint;
ALTER TABLE grading_period_groups ADD COLUMN IF NOT EXISTS display_totals_for_all_grading_periods boolean DEFAULT false;

-- ── grading_standards ─────────────────────────────────────────────────────────

ALTER TABLE grading_standards ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── submission_comments ───────────────────────────────────────────────────────

ALTER TABLE submission_comments ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE submission_comments ADD COLUMN IF NOT EXISTS hidden boolean DEFAULT false;
ALTER TABLE submission_comments ADD COLUMN IF NOT EXISTS group_comment_id text;

-- ── submissions ───────────────────────────────────────────────────────────────

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── assignment_groups ─────────────────────────────────────────────────────────

ALTER TABLE assignment_groups ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ── assignments ───────────────────────────────────────────────────────────────

ALTER TABLE assignments ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS peer_reviews boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS turnitin_enabled boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS post_to_sis boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS only_visible_to_overrides boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS omit_from_final_grade boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS muted boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS grade_group_students_individually boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS automatic_peer_reviews boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS allowed_extensions text;

COMMIT;
