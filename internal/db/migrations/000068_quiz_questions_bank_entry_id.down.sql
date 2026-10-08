DROP INDEX IF EXISTS idx_quiz_questions_question_bank_entry_id;

ALTER TABLE quiz_questions
    DROP COLUMN IF EXISTS question_bank_entry_id;
