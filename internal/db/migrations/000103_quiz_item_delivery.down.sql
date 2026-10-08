ALTER TABLE quiz_submissions DROP COLUMN IF EXISTS read_questions;

ALTER TABLE quizzes DROP COLUMN IF EXISTS anonymous_submissions;
ALTER TABLE quizzes DROP COLUMN IF EXISTS ip_filter;
ALTER TABLE quizzes DROP COLUMN IF EXISTS access_code_encrypted;
