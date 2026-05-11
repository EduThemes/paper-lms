-- Quiz settings imported from Canvas assessment_meta.xml. AutoMigrate handles
-- these in dev; production runs golang-migrate against this file.
ALTER TABLE quizzes
    ADD COLUMN IF NOT EXISTS shuffle_answers       boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS scoring_policy        text    NOT NULL DEFAULT 'keep_highest',
    ADD COLUMN IF NOT EXISTS show_correct_answers  boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS hide_results          text    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS one_question_at_a_time boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS cant_go_back          boolean NOT NULL DEFAULT false;
