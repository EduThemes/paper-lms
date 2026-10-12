-- 000079_grading_policy_columns.up.sql
--
-- Grading-policy parity wave (G018/G019/G022/G039).
--
-- submissions.late_policy_status / seconds_late_override: an instructor
-- can mark a submission late, missing, extended or none; the manual
-- status overrides the computed flags (Canvas late_policy_status).
-- submissions.cached_due_date: the student's effective due date
-- (overrides + accommodations), refreshed whenever dates change so
-- late / missing follow due-date edits.
-- courses.post_manually: course default post policy.
-- courses.grading_standard_id / grading_standard_disabled: choose (or turn
-- off) the course grading scheme instead of "newest scheme wins".
-- assignments.only_visible_to_overrides: assign to specific students or
-- sections only. assignments.grading_standard_id: per-assignment scheme.
-- grading_standards.points_based / scaling_factor: points-based schemes.

BEGIN;

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS late_policy_status text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS seconds_late_override bigint;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS cached_due_date timestamptz;
ALTER TABLE submissions DROP CONSTRAINT IF EXISTS chk_submissions_late_policy_status;
ALTER TABLE submissions ADD CONSTRAINT chk_submissions_late_policy_status
    CHECK (late_policy_status IS NULL OR late_policy_status IN ('none', 'missing', 'late', 'extended'));

ALTER TABLE courses ADD COLUMN IF NOT EXISTS post_manually boolean DEFAULT false;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS grading_standard_id bigint;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS grading_standard_disabled boolean DEFAULT false;

ALTER TABLE assignments ADD COLUMN IF NOT EXISTS only_visible_to_overrides boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS grading_standard_id bigint;

ALTER TABLE grading_standards ADD COLUMN IF NOT EXISTS points_based boolean DEFAULT false;
ALTER TABLE grading_standards ADD COLUMN IF NOT EXISTS scaling_factor numeric DEFAULT 1;

-- Existing rows start from the assignment's own due date; the next date
-- change (or override edit) recomputes per-student dates.
UPDATE submissions s
   SET cached_due_date = a.due_at
  FROM assignments a
 WHERE a.id = s.assignment_id
   AND s.cached_due_date IS NULL
   AND a.due_at IS NOT NULL;

COMMIT;
