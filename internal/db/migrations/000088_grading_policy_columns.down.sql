-- 000088_grading_policy_columns.down.sql

BEGIN;

ALTER TABLE grading_standards DROP COLUMN IF EXISTS scaling_factor;
ALTER TABLE grading_standards DROP COLUMN IF EXISTS points_based;

ALTER TABLE assignments DROP COLUMN IF EXISTS grading_standard_id;
ALTER TABLE assignments DROP COLUMN IF EXISTS only_visible_to_overrides;

ALTER TABLE courses DROP COLUMN IF EXISTS grading_standard_disabled;
ALTER TABLE courses DROP COLUMN IF EXISTS grading_standard_id;
ALTER TABLE courses DROP COLUMN IF EXISTS post_manually;

ALTER TABLE submissions DROP CONSTRAINT IF EXISTS chk_submissions_late_policy_status;
ALTER TABLE submissions DROP COLUMN IF EXISTS cached_due_date;
ALTER TABLE submissions DROP COLUMN IF EXISTS seconds_late_override;
ALTER TABLE submissions DROP COLUMN IF EXISTS late_policy_status;

COMMIT;
