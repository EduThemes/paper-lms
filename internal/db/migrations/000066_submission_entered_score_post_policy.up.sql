-- 000066_submission_entered_score_post_policy.up.sql
--
-- Grading parity with Canvas.
--
-- 1. entered_score / entered_grade / points_deducted (Canvas columns of
--    the same names). The grader's typed value is kept separately from
--    the late-policy-deducted score, so re-saving, curving or
--    re-importing a grade never applies the late penalty twice.
--
-- 2. Post-policy backfill. Before this change grading never set
--    posted_at, so every grade on an automatically-posted assignment was
--    technically "unposted". Grades are now hidden from students until
--    posted, so post the ones students could already see.

BEGIN;

ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS entered_score double precision,
    ADD COLUMN IF NOT EXISTS entered_grade text,
    ADD COLUMN IF NOT EXISTS points_deducted double precision;

-- Legacy rows only have the final score; treat it as the entered value.
UPDATE submissions
   SET entered_score = score,
       entered_grade = grade
 WHERE entered_score IS NULL
   AND score IS NOT NULL;

UPDATE submissions s
   SET posted_at = COALESCE(s.graded_at, s.updated_at, now())
  FROM assignments a
 WHERE s.assignment_id = a.id
   AND s.posted_at IS NULL
   AND (s.score IS NOT NULL OR s.excused)
   AND COALESCE(a.post_policy, 'automatic') <> 'manual';

COMMIT;
