-- 000068_quiz_questions_bank_entry_id.up.sql
--
-- Bank-linked question groups used to put question_bank_entries.id values
-- straight into a submission's selected_questions list, which is resolved
-- against quiz_questions — serving an unrelated (possibly cross-tenant)
-- quiz question, or silently dropping the draw. Drawn bank entries are now
-- materialized into the quiz as quiz_questions rows; this column records
-- which bank entry a row was copied from so later attempts reuse it.

ALTER TABLE quiz_questions
    ADD COLUMN IF NOT EXISTS question_bank_entry_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_quiz_questions_question_bank_entry_id
    ON quiz_questions (question_bank_entry_id);
