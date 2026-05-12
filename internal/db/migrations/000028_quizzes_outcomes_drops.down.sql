-- Reverse of 000028: re-add the dropped columns with their original types
-- and defaults. Structural only — the row data is LOST.
--
-- IMPORTANT: rows that existed when 000028 ran no longer have legacy-column
-- values. After this down runs, all re-added columns are NULL/default. If you
-- need the legacy data back, you must restore from a backup taken before
-- 000028 ran; the .up.sql is data-destructive by design.
--
-- Original types sourced from:
--   000001_init.up.sql       — quizzes, quiz_questions, quiz_submissions,
--                              quiz_submission_answers, learning_outcome_groups,
--                              learning_outcomes, learning_outcome_results,
--                              outcome_alignments
--   000006_mastery_paths.up.sql  — conditional_release_* (no drops here)
--   000015_quiz_banks_stimulus.up.sql — quiz_item_banks, quiz_stimuli (no drops)
--   000016_backfill_missing_tables.up.sql — question_banks, quiz_question_groups
--
-- NOT NULL constraints that would fail on populated tables are re-added as
-- nullable. Operators rolling back are presumably also restoring data and can
-- re-tighten constraints separately.

BEGIN;

-- ============================================================================
-- outcome_alignments
-- ============================================================================

ALTER TABLE outcome_alignments ADD COLUMN IF NOT EXISTS context_type text;
ALTER TABLE outcome_alignments ADD COLUMN IF NOT EXISTS context_id bigint;
-- original NOT NULL columns re-added as nullable to avoid failure on populated tables
ALTER TABLE outcome_alignments ADD COLUMN IF NOT EXISTS content_type text;
ALTER TABLE outcome_alignments ADD COLUMN IF NOT EXISTS content_id bigint;
ALTER TABLE outcome_alignments ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- learning_outcome_results
-- ============================================================================

ALTER TABLE learning_outcome_results ADD COLUMN IF NOT EXISTS artifact_type text;
ALTER TABLE learning_outcome_results ADD COLUMN IF NOT EXISTS artifact_id bigint;
ALTER TABLE learning_outcome_results ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- learning_outcomes
-- ============================================================================

ALTER TABLE learning_outcomes ADD COLUMN IF NOT EXISTS vendor_guid text;
ALTER TABLE learning_outcomes ADD COLUMN IF NOT EXISTS ratings text;
ALTER TABLE learning_outcomes ADD COLUMN IF NOT EXISTS learning_outcome_group_id bigint;
ALTER TABLE learning_outcomes ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- learning_outcome_groups
-- ============================================================================

ALTER TABLE learning_outcome_groups ADD COLUMN IF NOT EXISTS vendor_guid text;
ALTER TABLE learning_outcome_groups ADD COLUMN IF NOT EXISTS parent_outcome_group_id bigint;
ALTER TABLE learning_outcome_groups ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- quiz_submission_answers
-- ============================================================================

ALTER TABLE quiz_submission_answers ADD COLUMN IF NOT EXISTS quiz_question_id bigint;
ALTER TABLE quiz_submission_answers ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- quiz_submissions
-- ============================================================================

ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS manually_unlocked boolean DEFAULT false;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS fudge_points double precision DEFAULT 0;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS extra_time bigint DEFAULT 0;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS extra_attempts bigint DEFAULT 0;
ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- quiz_questions
-- ============================================================================

ALTER TABLE quiz_questions ADD COLUMN IF NOT EXISTS question_name text;
ALTER TABLE quiz_questions ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- quizzes
-- ============================================================================

ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS show_correct_answers_at timestamptz;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS ip_filter text;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS hide_correct_answers_at timestamptz;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS assignment_id bigint;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS assignment_group_id bigint;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS access_code text;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

COMMIT;
