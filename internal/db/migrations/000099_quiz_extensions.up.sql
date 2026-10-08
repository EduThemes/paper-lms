-- 000099_quiz_extensions.up.sql
--
-- G047: per-student quiz moderation (Canvas "Moderate this quiz"): extra
-- attempts, extra minutes per timed attempt, and manual unlock. Replaces
-- the extra_attempts / extra_time / manually_unlocked columns dropped
-- from quiz_submissions in 000028 with one row per (quiz, user).

BEGIN;

CREATE TABLE IF NOT EXISTS quiz_extensions (
    id                bigserial PRIMARY KEY,
    quiz_id           bigint  NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    user_id           bigint  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    extra_attempts    bigint  NOT NULL DEFAULT 0,
    extra_time        bigint  NOT NULL DEFAULT 0,
    manually_unlocked boolean NOT NULL DEFAULT false,
    created_at        timestamptz,
    updated_at        timestamptz,
    CONSTRAINT quiz_extensions_extra_attempts_check CHECK (extra_attempts >= 0),
    CONSTRAINT quiz_extensions_extra_time_check CHECK (extra_time >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quiz_extensions_quiz_user
    ON quiz_extensions (quiz_id, user_id);
CREATE INDEX IF NOT EXISTS idx_quiz_extensions_user_id
    ON quiz_extensions (user_id);

COMMIT;
