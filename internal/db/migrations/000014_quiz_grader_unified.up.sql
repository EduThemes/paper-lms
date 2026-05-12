-- Wave A1: add graded_via audit column to quiz_submission_answers.
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
--
-- Note on table existence: quiz_submission_answers is provisioned by GORM
-- AutoMigrate (see internal/db/postgres.go) rather than the SQL migration
-- chain. In prod-mode deploys the table will exist by the time this runs
-- (after the migration backfill scheduled separately). In dev-mode runs the
-- ALTER is skipped here and AutoMigrate picks up the GradedVia field directly
-- from the model definition. The guard makes the migration safe in both.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema()
          AND table_name = 'quiz_submission_answers'
    ) THEN
        ALTER TABLE quiz_submission_answers
            ADD COLUMN IF NOT EXISTS graded_via VARCHAR(32) DEFAULT NULL;
    END IF;
END
$$;
