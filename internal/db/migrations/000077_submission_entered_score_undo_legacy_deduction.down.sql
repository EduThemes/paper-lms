-- 000077_submission_entered_score_undo_legacy_deduction.down.sql
--
-- Data repair only; nothing to undo. The recovered entered_score values
-- are the grader's typed values, which is what the columns mean, and
-- rolling 000066 back drops the columns anyway.
SELECT 1;
