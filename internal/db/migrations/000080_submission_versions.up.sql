-- 000080_submission_versions.up.sql
--
-- Submission history (G028): a resubmission used to overwrite the body,
-- URL and attachments in place, losing earlier attempts. Each attempt is
-- now kept as a submission_versions row (one per submission + attempt),
-- and its grade fields follow the grading of that attempt.

BEGIN;

CREATE TABLE IF NOT EXISTS submission_versions (
    id bigserial,
    submission_id bigint NOT NULL,
    attempt bigint NOT NULL,
    assignment_id bigint NOT NULL,
    user_id bigint NOT NULL,
    submission_type text,
    body text,
    url text,
    attachments text,
    submitted_at timestamptz,
    late boolean DEFAULT false,
    score numeric,
    grade text,
    entered_score numeric,
    grader_id bigint,
    graded_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_submission_versions_submission_attempt ON public.submission_versions USING btree (submission_id, attempt);
CREATE INDEX IF NOT EXISTS idx_submission_versions_assignment_id ON public.submission_versions USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_submission_versions_user_id ON public.submission_versions USING btree (user_id);

-- Seed the current attempt of every submitted row so history starts with
-- what is already there.
INSERT INTO submission_versions (submission_id, attempt, assignment_id, user_id, submission_type, body, url,
                                 attachments, submitted_at, late, score, grade, entered_score, grader_id,
                                 graded_at, created_at, updated_at)
SELECT s.id, GREATEST(s.attempt, 1), s.assignment_id, s.user_id, s.submission_type, s.body, s.url,
       s.attachments, s.submitted_at, COALESCE(s.late, false), s.score, s.grade, s.entered_score, s.grader_id,
       s.graded_at, now(), now()
  FROM submissions s
 WHERE s.submitted_at IS NOT NULL
ON CONFLICT DO NOTHING;

COMMIT;
