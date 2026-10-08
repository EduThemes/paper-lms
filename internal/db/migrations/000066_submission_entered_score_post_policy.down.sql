-- The posted_at backfill is not reversible (it only released grades that
-- were already visible before 000066); only the new columns are dropped.
BEGIN;

ALTER TABLE submissions
    DROP COLUMN IF EXISTS points_deducted,
    DROP COLUMN IF EXISTS entered_grade,
    DROP COLUMN IF EXISTS entered_score;

COMMIT;
