DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema()
          AND table_name = 'quiz_submission_answers'
    ) THEN
        ALTER TABLE quiz_submission_answers
            DROP COLUMN IF EXISTS graded_via;
    END IF;
END
$$;
