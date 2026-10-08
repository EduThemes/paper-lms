-- 000116_student_view_students.up.sql
--
-- Student View (G076). Each course has at most one "Test Student": a
-- regular user with an active StudentEnrollment, marked fake by its row
-- here. Staff enter Student View by switching their session to that user.

CREATE TABLE IF NOT EXISTS student_view_students (
    id bigserial,
    course_id bigint NOT NULL,
    user_id bigint NOT NULL,
    created_at timestamptz,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_student_view_students_course_id ON public.student_view_students USING btree (course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_student_view_students_user_id ON public.student_view_students USING btree (user_id);
