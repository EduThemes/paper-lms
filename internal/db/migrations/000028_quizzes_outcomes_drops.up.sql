-- Wave 2c: drop legacy columns on quizzes + outcomes domain. DATA-DESTRUCTIVE.
--
-- Deprecation window: added 2026-05-11. Operators with production data that
-- predates this migration MUST have run 000021 first; that migration copies
-- legacy data into the new GORM-model columns. Once this migration runs the
-- dropped columns are gone — the .down.sql can recreate the shape but not
-- the data.
--
-- Domain tables: quizzes, quiz_questions, quiz_submissions,
--   quiz_submission_answers, question_banks (clean), question_bank_entries
--   (clean), quiz_question_groups (clean), quiz_item_banks (clean),
--   quiz_item_bank_items (clean), quiz_stimuli (clean),
--   quiz_question_outcome_alignments (clean), learning_outcome_groups,
--   learning_outcomes, learning_outcome_results, outcome_alignments,
--   outcome_proficiencies (clean), outcome_proficiency_ratings (clean),
--   conditional_release_* tables (all clean — 5 tables).
--
-- Per-table change summary:
--
--   quizzes (7):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     access_code              — UNKNOWN / dead column, zero non-test refs
--     assignment_group_id      — UNKNOWN / dead on quizzes (model has none)
--     assignment_id            — UNKNOWN / dead on quizzes (model has none)
--     hide_correct_answers_at  — UNKNOWN / dead column, zero refs
--     ip_filter                — UNKNOWN / dead column, zero refs
--     show_correct_answers_at  — UNKNOWN / dead column, zero refs
--
--   quiz_questions (2):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     question_name            — UNKNOWN / model has no such field; the
--                                only ref (question_bank.go:19) is on
--                                QuestionBankEntry, not QuizQuestion
--
--   quiz_submissions (5):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     extra_attempts           — UNKNOWN / dead column, zero refs
--     extra_time               — UNKNOWN / dead column, zero refs
--     fudge_points             — UNKNOWN / dead column, zero refs
--     manually_unlocked        — UNKNOWN / dead column, zero refs
--
--   quiz_submission_answers (2):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     quiz_question_id         — Wave 2b source (data now in question_id)
--
--   learning_outcome_groups (3):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     parent_outcome_group_id  — Wave 2b source (data now in parent_group_id)
--     vendor_guid              — UNKNOWN / dead column, zero refs
--
--   learning_outcomes (4):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     learning_outcome_group_id — Wave 2b source (data now in outcome_group_id)
--     ratings                  — Wave 2b source (data now in ratings_data)
--     vendor_guid              — UNKNOWN / dead column, zero refs
--
--   learning_outcome_results (3):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     artifact_id              — POLYMORPHIC_REFACTOR / dead on this model;
--                                all artifact_id refs are for portfolio tables
--     artifact_type            — UNKNOWN / dead on this model; ditto
--
--   outcome_alignments (5):
--     deleted_at               — SOFT_DELETE_LEFTOVER
--     content_id               — UNKNOWN / dead; model uses assignment_id
--     content_type             — UNKNOWN / dead; model uses no content_type
--     context_id               — UNKNOWN / dead; model has no context fields
--     context_type             — UNKNOWN / dead; model has no context fields
--
-- KEPT (all have live GORM model fields):
--   (no columns were kept — every stale column was either a Wave 2b copy
--    source or a dead UNKNOWN with zero non-test refs in the quizzes/outcomes
--    domain; see grep verification notes in commit message)

BEGIN;

-- ============================================================================
-- quizzes
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE quizzes DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / dead columns on quizzes
ALTER TABLE quizzes DROP COLUMN IF EXISTS access_code;
ALTER TABLE quizzes DROP COLUMN IF EXISTS assignment_group_id;
ALTER TABLE quizzes DROP COLUMN IF EXISTS assignment_id;
ALTER TABLE quizzes DROP COLUMN IF EXISTS hide_correct_answers_at;
ALTER TABLE quizzes DROP COLUMN IF EXISTS ip_filter;
ALTER TABLE quizzes DROP COLUMN IF EXISTS show_correct_answers_at;

-- ============================================================================
-- quiz_questions
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE quiz_questions DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / the only codebase ref is QuestionBankEntry.QuestionName (question_bank.go:19),
-- not QuizQuestion — dead column on this table
ALTER TABLE quiz_questions DROP COLUMN IF EXISTS question_name;

-- ============================================================================
-- quiz_submissions
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / dead columns — zero non-test Go refs
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS extra_attempts;
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS extra_time;
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS fudge_points;
ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS manually_unlocked;

-- ============================================================================
-- quiz_submission_answers
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE quiz_submission_answers DROP COLUMN IF EXISTS deleted_at;

-- Wave 2b source: data copied to question_id in 000021
ALTER TABLE quiz_submission_answers DROP COLUMN IF EXISTS quiz_question_id;

-- ============================================================================
-- learning_outcome_groups
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE learning_outcome_groups DROP COLUMN IF EXISTS deleted_at;

-- Wave 2b source: data copied to parent_group_id in 000021
ALTER TABLE learning_outcome_groups DROP COLUMN IF EXISTS parent_outcome_group_id;

-- UNKNOWN / dead column — zero refs
ALTER TABLE learning_outcome_groups DROP COLUMN IF EXISTS vendor_guid;

-- ============================================================================
-- learning_outcomes
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE learning_outcomes DROP COLUMN IF EXISTS deleted_at;

-- Wave 2b source: data copied to outcome_group_id in 000021
ALTER TABLE learning_outcomes DROP COLUMN IF EXISTS learning_outcome_group_id;

-- Wave 2b source: data copied to ratings_data (jsonb) in 000021
ALTER TABLE learning_outcomes DROP COLUMN IF EXISTS ratings;

-- UNKNOWN / dead column — zero refs
ALTER TABLE learning_outcomes DROP COLUMN IF EXISTS vendor_guid;

-- ============================================================================
-- learning_outcome_results
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE learning_outcome_results DROP COLUMN IF EXISTS deleted_at;

-- POLYMORPHIC_REFACTOR / dead on LearningOutcomeResult model; all artifact_id
-- refs in codebase belong to portfolio_artifacts / portfolio_reflections /
-- portfolio_comments — not this table
ALTER TABLE learning_outcome_results DROP COLUMN IF EXISTS artifact_id;

-- UNKNOWN / dead on this model for the same reason
ALTER TABLE learning_outcome_results DROP COLUMN IF EXISTS artifact_type;

-- ============================================================================
-- outcome_alignments
-- ============================================================================

-- SOFT_DELETE_LEFTOVER
ALTER TABLE outcome_alignments DROP COLUMN IF EXISTS deleted_at;

-- UNKNOWN / dead: model replaced these with assignment_id + course_id (000016)
ALTER TABLE outcome_alignments DROP COLUMN IF EXISTS content_id;
ALTER TABLE outcome_alignments DROP COLUMN IF EXISTS content_type;
ALTER TABLE outcome_alignments DROP COLUMN IF EXISTS context_id;
ALTER TABLE outcome_alignments DROP COLUMN IF EXISTS context_type;

COMMIT;
