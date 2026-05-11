-- Wave A1: add graded_via audit column to quiz_submission_answer.
--
-- This column records *how* a given answer row was scored. Values are written
-- by the auto-grader service layer:
--   'auto'   — scored by the auto-grader (multiple_choice, matching,
--              fill_in_multiple_blanks, numerical_question with margin, …)
--   'manual' — scored later by an instructor in the SpeedGrader-style flow
--   NULL     — legacy: pre-Wave A1 rows have never been audit-stamped.
--
-- Backwards-compat invariant: this migration does NOT retroactively populate
-- legacy rows. Submissions already in `pending_review` from the old
-- matching / fill_in_multiple_blanks code path stay exactly as they are.
-- The auto-grader only stamps NEW rows. See quiz_service.go autoGrade().
ALTER TABLE quiz_submission_answer
    ADD COLUMN IF NOT EXISTS graded_via VARCHAR(32) DEFAULT NULL;
