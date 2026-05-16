-- Reverse of 000043. Pseudonym mappings are lost on rollback; learners
-- get a fresh roll the next time they view a leaderboard after re-apply.

BEGIN;

DROP INDEX IF EXISTS idx_enrollments_pseudonym_unique_per_course;

ALTER TABLE enrollments
    DROP COLUMN IF EXISTS pseudonym_name,
    DROP COLUMN IF EXISTS pseudonym_pool_code;

COMMIT;
