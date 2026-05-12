-- Reverse the data copies from the .up.sql. Each statement back-populates
-- the legacy column from the new one when the legacy column is at its zero
-- value, mirroring the up direction.
--
-- Lossy by design for the text→jsonb cast (ratings → ratings_data): the
-- jsonb round-trip may normalise whitespace/key order. That is acceptable —
-- Wave 2b is additive, and Wave 2c's drop migration removes legacy columns.

BEGIN;

-- Reverse 4: question_id → quiz_question_id.
UPDATE quiz_submission_answers
SET quiz_question_id = question_id
WHERE (quiz_question_id IS NULL OR quiz_question_id = 0)
  AND question_id > 0;

-- Reverse 3: ratings_data → ratings (jsonb → text cast).
UPDATE learning_outcomes
SET ratings = ratings_data::text
WHERE (ratings IS NULL OR ratings = '')
  AND ratings_data IS NOT NULL;

-- Reverse 2: outcome_group_id → learning_outcome_group_id.
UPDATE learning_outcomes
SET learning_outcome_group_id = outcome_group_id
WHERE (learning_outcome_group_id IS NULL OR learning_outcome_group_id = 0)
  AND outcome_group_id > 0;

-- Reverse 1: parent_group_id → parent_outcome_group_id.
UPDATE learning_outcome_groups
SET parent_outcome_group_id = parent_group_id
WHERE parent_outcome_group_id IS NULL
  AND parent_group_id IS NOT NULL;

COMMIT;
