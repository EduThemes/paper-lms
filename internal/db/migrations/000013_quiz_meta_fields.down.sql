ALTER TABLE quizzes
    DROP COLUMN IF EXISTS shuffle_answers,
    DROP COLUMN IF EXISTS scoring_policy,
    DROP COLUMN IF EXISTS show_correct_answers,
    DROP COLUMN IF EXISTS hide_results,
    DROP COLUMN IF EXISTS one_question_at_a_time,
    DROP COLUMN IF EXISTS cant_go_back;
