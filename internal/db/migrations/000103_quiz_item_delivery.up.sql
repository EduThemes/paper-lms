-- 000103_quiz_item_delivery.up.sql
--
-- Quiz delivery settings (Canvas parity, quiz-items lane):
--   * access_code_encrypted — the quiz access code, sealed with secretbox
--     (AES-GCM) like every other DB-resident secret. The plaintext
--     access_code column dropped in 000028 is not revived.
--   * ip_filter — comma-separated IP addresses / CIDR ranges a student
--     must take the quiz from.
--   * anonymous_submissions — survey responses are reported without the
--     student's identity.
--   * quiz_submissions.read_questions — JSON array of question ids a
--     student has moved past on a "can't go back" quiz; the server refuses
--     further answers to them.

ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS access_code_encrypted bytea;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS ip_filter text;
ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS anonymous_submissions boolean NOT NULL DEFAULT false;

ALTER TABLE quiz_submissions ADD COLUMN IF NOT EXISTS read_questions text;
