-- Wave 2b: data migration for the quizzes + outcomes domain.
--
-- Tables analysed (22 total in this domain):
--
--   Tables with COPY work (4 tables):
--     learning_outcome_groups   – parent_outcome_group_id → parent_group_id
--     learning_outcomes         – learning_outcome_group_id → outcome_group_id
--                                 ratings → ratings_data (text → jsonb cast)
--     quiz_submission_answers   – quiz_question_id → question_id
--
--   No-op tables (all stale cols are SOFT_DELETE_LEFTOVER or dead UNKNOWN):
--     quizzes                   – deleted_at (SOFT_DELETE), access_code,
--                                 assignment_group_id, assignment_id,
--                                 hide_correct_answers_at, ip_filter,
--                                 show_correct_answers_at (no model field)
--     quiz_questions            – deleted_at (SOFT_DELETE), question_name
--                                 (dead: model has no such field; the
--                                 question_bank.go:19 ref is QuestionBankEntry)
--     quiz_submissions          – deleted_at (SOFT_DELETE), extra_attempts,
--                                 extra_time, fudge_points, manually_unlocked
--                                 (no model field)
--     quiz_submission_answers   – deleted_at (SOFT_DELETE) deferred;
--                                 quiz_question_id COPY handled above
--     question_banks            – not listed in STALE_COLUMNS
--     question_bank_entries     – not listed in STALE_COLUMNS
--     quiz_question_groups      – not listed in STALE_COLUMNS
--     quiz_item_banks           – not listed in STALE_COLUMNS
--     quiz_item_bank_items      – not listed in STALE_COLUMNS
--     quiz_stimuli              – not listed in STALE_COLUMNS
--     quiz_question_outcome_alignments – not listed in STALE_COLUMNS
--     learning_outcome_results  – deleted_at (SOFT_DELETE), artifact_id and
--                                 artifact_type are separate stale columns;
--                                 associated_asset_id / associated_asset_type
--                                 already exist in old schema → DEFER
--     outcome_alignments        – deleted_at (SOFT_DELETE), content_id,
--                                 content_type, context_id, context_type
--                                 (no model field; dead columns) → DEFER
--     outcome_proficiencies     – not listed in STALE_COLUMNS
--     outcome_proficiency_ratings – not listed in STALE_COLUMNS
--     conditional_release_rules – not listed in STALE_COLUMNS
--     conditional_release_scoring_ranges – not listed in STALE_COLUMNS
--     conditional_release_assignment_sets – not listed in STALE_COLUMNS
--     conditional_release_assignment_set_associations – not listed in STALE_COLUMNS
--     conditional_release_assignment_set_actions – not listed in STALE_COLUMNS
--
-- Re-classified UNKNOWNs:
--   learning_outcome_groups.parent_outcome_group_id → RENAME to parent_group_id
--   learning_outcomes.learning_outcome_group_id     → RENAME to outcome_group_id
--   learning_outcomes.ratings                       → RENAME to ratings_data
--   quiz_submission_answers.quiz_question_id        → RENAME to question_id
--
-- All guards use GORM zero-values ('' for text, 0 for bigint, NULL for
-- nullable types). Every statement is idempotent.

BEGIN;

-- ============================================================================
-- learning_outcome_groups
-- ============================================================================

-- 1. parent_outcome_group_id → parent_group_id (direct rename, both nullable).
UPDATE learning_outcome_groups
SET parent_group_id = parent_outcome_group_id
WHERE parent_group_id IS NULL
  AND parent_outcome_group_id IS NOT NULL;

-- ============================================================================
-- learning_outcomes
-- ============================================================================

-- 2. learning_outcome_group_id → outcome_group_id (rename; new col is NOT NULL
--    bigint so guard on zero-value 0).
UPDATE learning_outcomes
SET outcome_group_id = learning_outcome_group_id
WHERE outcome_group_id = 0
  AND learning_outcome_group_id IS NOT NULL
  AND learning_outcome_group_id > 0;

-- 3. ratings (text) → ratings_data (jsonb). Cast via explicit CAST so the
--    stored text JSON is interpreted as jsonb without double-encoding.
--    Guard: ratings_data IS NULL AND ratings IS NOT NULL AND ratings <> ''.
UPDATE learning_outcomes
SET ratings_data = ratings::jsonb
WHERE ratings_data IS NULL
  AND ratings IS NOT NULL
  AND ratings <> '';

-- ============================================================================
-- quiz_submission_answers
-- ============================================================================

-- 4. quiz_question_id → question_id (rename; new col is NOT NULL bigint so
--    guard on zero-value 0).
UPDATE quiz_submission_answers
SET question_id = quiz_question_id
WHERE question_id = 0
  AND quiz_question_id IS NOT NULL
  AND quiz_question_id > 0;

COMMIT;
