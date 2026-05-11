-- Reverse Wave A2 additions.

DROP TABLE IF EXISTS quiz_question_outcome_alignments;

DROP INDEX IF EXISTS idx_quiz_questions_stimulus_id;
DROP INDEX IF EXISTS idx_quiz_questions_bank_item_id;
ALTER TABLE quiz_questions
    DROP COLUMN IF EXISTS stimulus_id,
    DROP COLUMN IF EXISTS bank_item_id;

DROP TABLE IF EXISTS quiz_stimuli;
DROP TABLE IF EXISTS quiz_item_bank_items;
DROP TABLE IF EXISTS quiz_item_banks;
