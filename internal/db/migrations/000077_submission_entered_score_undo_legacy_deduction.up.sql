-- 000077_submission_entered_score_undo_legacy_deduction.up.sql
--
-- 000066 backfilled entered_score = score for every graded row. Before it,
-- the late penalty was written straight into submissions.score and the
-- grader's typed value was discarded, so on legacy late rows "score" is
-- already deducted. Grading now always deducts from entered_score, so any
-- re-save (SpeedGrader "Update Grade", a curve, a CSV import) would take
-- the penalty a second time (10 typed, 20% late -> stored 8 -> re-saved 6.4).
--
-- Recover the entered value by inverting the old formula
--     score = entered * (1 - least(intervals * deduction, 100) / 100)
-- for rows that the old code deducted: late, graded, submitted after the
-- assignment's due date, in a course whose late policy deducts, and still
-- carrying the 000066 backfill (entered_score = score, no points_deducted).
-- Rows where the old penalty reached 100% or hit the minimum-percent floor
-- can't be inverted and are left as they are.
--
-- entered_grade is only rewritten for points-graded assignments, where the
-- grade string is the number itself. For letter_grade / percent / pass_fail /
-- gpa_scale assignments it holds the grader's display value ("B-", "85%"),
-- which a raw number would clobber, so it is left as it is.

WITH legacy AS (
    SELECT s.id,
           s.score,
           COALESCE(NULLIF(a.grading_type, ''), 'points') AS grading_type,
           LEAST(
               CEIL(EXTRACT(EPOCH FROM (s.submitted_at - a.due_at))
                    / CASE WHEN lp.late_submission_interval = 'hour' THEN 3600.0 ELSE 86400.0 END)
               * lp.late_submission_deduction,
               100) AS pct,
           CASE
               WHEN lp.late_submission_minimum_percent_enabled AND a.points_possible > 0
                   THEN a.points_possible * lp.late_submission_minimum_percent / 100.0
           END AS floor_score
      FROM submissions s
      JOIN assignments a   ON a.id = s.assignment_id
      JOIN late_policies lp ON lp.course_id = a.course_id
     WHERE s.late
       AND s.score IS NOT NULL
       AND s.score > 0
       AND s.entered_score = s.score
       AND s.points_deducted IS NULL
       AND s.submitted_at IS NOT NULL
       AND a.due_at IS NOT NULL
       AND s.submitted_at > a.due_at
       AND lp.late_submission_deduction_enabled
       AND lp.late_submission_deduction > 0
), recovered AS (
    SELECT id,
           score,
           grading_type,
           ROUND((score / (1 - pct / 100.0))::numeric, 6) AS entered
      FROM legacy
     WHERE pct < 100
       AND (floor_score IS NULL OR score > floor_score + 0.000001)
)
UPDATE submissions s
   SET entered_score   = r.entered::double precision,
       entered_grade   = CASE
                             WHEN r.grading_type = 'points'
                                 THEN RTRIM(RTRIM(r.entered::text, '0'), '.')
                             ELSE s.entered_grade
                         END,
       points_deducted = ROUND((r.entered - r.score::numeric), 6)::double precision
  FROM recovered r
 WHERE s.id = r.id;
