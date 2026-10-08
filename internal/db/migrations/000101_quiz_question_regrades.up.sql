-- 000101_quiz_question_regrades.up.sql
--
-- G053: regrade on answer-key change. Each row is one regrade run on a
-- question: the option (current correct only / award both / full credit /
-- no regrade), the answer key it applied, who ran it, and how many
-- finished attempts were rescored and changed.

BEGIN;

CREATE TABLE IF NOT EXISTS quiz_question_regrades (
    id                   bigserial PRIMARY KEY,
    quiz_id              bigint NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    quiz_question_id     bigint NOT NULL,
    regrade_option       text   NOT NULL,
    answers_snapshot     text,
    regraded_by_id       bigint NOT NULL,
    submissions_regraded bigint NOT NULL DEFAULT 0,
    submissions_changed  bigint NOT NULL DEFAULT 0,
    created_at           timestamptz,
    updated_at           timestamptz,
    CONSTRAINT quiz_question_regrades_option_check CHECK (regrade_option IN
        ('current_correct_only', 'current_and_previous_correct', 'full_credit', 'no_regrade'))
);

CREATE INDEX IF NOT EXISTS idx_quiz_question_regrades_quiz
    ON quiz_question_regrades (quiz_id);

COMMIT;
