-- 000090_course_score_overrides.up.sql
--
-- Final grade override (G029): an instructor can override a student's
-- computed course grade (or one grading period's). grading_period_id 0
-- is the course-level override.

BEGIN;

CREATE TABLE IF NOT EXISTS course_score_overrides (
    id bigserial,
    course_id bigint NOT NULL,
    user_id bigint NOT NULL,
    grading_period_id bigint NOT NULL DEFAULT 0,
    override_score numeric NOT NULL,
    updated_by_id bigint,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_course_score_overrides_scope ON public.course_score_overrides USING btree (course_id, user_id, grading_period_id);
CREATE INDEX IF NOT EXISTS idx_course_score_overrides_user_id ON public.course_score_overrides USING btree (user_id);

COMMIT;
