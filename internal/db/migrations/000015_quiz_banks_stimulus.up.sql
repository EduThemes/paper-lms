-- Wave A2: Quiz item banks, stimulus passages, outcome alignment per question.
-- All additive — existing quiz, quiz_question, and grading tables are unaffected.

-- ------------------------------------------------------------------
-- Quiz Item Banks (course-scoped library of reusable question templates)
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS quiz_item_banks (
    id                  BIGSERIAL PRIMARY KEY,
    course_id           BIGINT       NOT NULL,
    title               VARCHAR(255) NOT NULL,
    description         TEXT         NOT NULL DEFAULT '',
    created_by_user_id  BIGINT       NOT NULL,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quiz_item_banks_course_id
    ON quiz_item_banks (course_id);
CREATE INDEX IF NOT EXISTS idx_quiz_item_banks_created_by
    ON quiz_item_banks (created_by_user_id);

-- ------------------------------------------------------------------
-- Quiz Item Bank Items (reusable question templates; mirror QuizQuestion)
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS quiz_item_bank_items (
    id                  BIGSERIAL PRIMARY KEY,
    bank_id             BIGINT       NOT NULL,
    position            INTEGER      NOT NULL DEFAULT 0,
    question_type       TEXT         NOT NULL,
    question_text       TEXT         NOT NULL,
    points_possible     DOUBLE PRECISION,
    answers             JSONB        NOT NULL DEFAULT '[]'::jsonb,
    correct_comments    TEXT         NOT NULL DEFAULT '',
    incorrect_comments  TEXT         NOT NULL DEFAULT '',
    neutral_comments    TEXT         NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quiz_item_bank_items_bank_id
    ON quiz_item_bank_items (bank_id);

-- ------------------------------------------------------------------
-- Stimulus passages (TipTap doc shared across multiple quiz_questions)
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS quiz_stimuli (
    id          BIGSERIAL PRIMARY KEY,
    course_id   BIGINT       NOT NULL,
    title       VARCHAR(255) NOT NULL,
    content     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_quiz_stimuli_course_id
    ON quiz_stimuli (course_id);

-- ------------------------------------------------------------------
-- quiz_questions: add nullable FKs for bank item and stimulus
-- ------------------------------------------------------------------
ALTER TABLE quiz_questions
    ADD COLUMN IF NOT EXISTS bank_item_id BIGINT,
    ADD COLUMN IF NOT EXISTS stimulus_id  BIGINT;
CREATE INDEX IF NOT EXISTS idx_quiz_questions_bank_item_id
    ON quiz_questions (bank_item_id);
CREATE INDEX IF NOT EXISTS idx_quiz_questions_stimulus_id
    ON quiz_questions (stimulus_id);

-- ------------------------------------------------------------------
-- Per-question outcome alignment with mastery threshold
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS quiz_question_outcome_alignments (
    id                  BIGSERIAL PRIMARY KEY,
    quiz_question_id    BIGINT           NOT NULL,
    outcome_id          BIGINT           NOT NULL,
    mastery_threshold   DOUBLE PRECISION NOT NULL DEFAULT 0.7,
    created_at          TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_quiz_question_outcome
        UNIQUE (quiz_question_id, outcome_id)
);
CREATE INDEX IF NOT EXISTS idx_qqoa_quiz_question_id
    ON quiz_question_outcome_alignments (quiz_question_id);
CREATE INDEX IF NOT EXISTS idx_qqoa_outcome_id
    ON quiz_question_outcome_alignments (outcome_id);
