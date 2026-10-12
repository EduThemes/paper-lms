-- 000083_moderated_grading.up.sql
--
-- Moderated grading (Canvas parity): graders who took part, their
-- provisional grades, and the moderator's selection per student.
CREATE TABLE IF NOT EXISTS moderation_graders (
    id bigserial,
    assignment_id bigint NOT NULL,
    user_id bigint NOT NULL,
    anonymous_id text NOT NULL,
    slot_taken boolean NOT NULL DEFAULT false,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_moderation_graders_assignment_user ON public.moderation_graders USING btree (assignment_id, user_id);
CREATE INDEX IF NOT EXISTS idx_moderation_graders_user_id ON public.moderation_graders USING btree (user_id);

CREATE TABLE IF NOT EXISTS moderated_grading_provisional_grades (
    id bigserial,
    submission_id bigint NOT NULL,
    scorer_id bigint NOT NULL,
    final boolean NOT NULL DEFAULT false,
    score numeric,
    grade text,
    graded_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_provisional_grades_sub_scorer_final ON public.moderated_grading_provisional_grades USING btree (submission_id, scorer_id, final);
CREATE INDEX IF NOT EXISTS idx_moderated_grading_provisional_grades_scorer_id ON public.moderated_grading_provisional_grades USING btree (scorer_id);

CREATE TABLE IF NOT EXISTS moderated_grading_selections (
    id bigserial,
    assignment_id bigint NOT NULL,
    student_id bigint NOT NULL,
    selected_provisional_grade_id bigint,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_moderation_selections_assignment_student ON public.moderated_grading_selections USING btree (assignment_id, student_id);
