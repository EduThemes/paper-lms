-- 000094_submission_versions.up.sql
--
-- One row per submitted attempt so SpeedGrader can show earlier attempts
-- (Canvas submission history). Written by SubmissionService on submit.
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
    late boolean NOT NULL DEFAULT false,
    created_at timestamptz,
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_submission_versions_submission_attempt ON public.submission_versions USING btree (submission_id, attempt);
CREATE INDEX IF NOT EXISTS idx_submission_versions_assignment_id ON public.submission_versions USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_submission_versions_user_id ON public.submission_versions USING btree (user_id);
