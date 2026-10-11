-- 000082_grading_ui_submission_settings.up.sql
--
-- Grading-UI lane: per-assignment submission limits (allowed attempts and
-- file extensions), the annotatable document for student_annotation
-- assignments, grading group members individually, moderated grading
-- settings, anonymous peer reviews and the peer review due date. Also the
-- per-submission anonymous id / extra attempts / custom grade status,
-- comment attachments + group comments and rubric-based peer reviews.
-- (submissions.late_policy_status comes from 000079 and
-- courses.hide_final_grades from 000124.) Several of these columns
-- existed before 000027 dropped them as unused; they come back now that
-- the features read them.

ALTER TABLE assignments ADD COLUMN IF NOT EXISTS allowed_attempts bigint;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS allowed_extensions text;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS annotatable_attachment_id bigint;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS grade_group_students_individually boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS moderated_grading boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS grader_count bigint DEFAULT 0;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS final_grader_id bigint;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS grader_comments_visible_to_graders boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS graders_anonymous_to_graders boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS grader_names_visible_to_final_grader boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS grades_published_at timestamptz;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS anonymous_peer_reviews boolean DEFAULT false;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS peer_reviews_due_at timestamptz;
CREATE INDEX IF NOT EXISTS idx_assignments_annotatable_attachment_id ON public.assignments USING btree (annotatable_attachment_id);

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS anonymous_id text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS extra_attempts bigint;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS custom_grade_status_id bigint;
CREATE INDEX IF NOT EXISTS idx_submissions_anonymous_id ON public.submissions USING btree (anonymous_id);
CREATE INDEX IF NOT EXISTS idx_submissions_custom_grade_status_id ON public.submissions USING btree (custom_grade_status_id);

ALTER TABLE submission_comments ADD COLUMN IF NOT EXISTS attachments text;
ALTER TABLE submission_comments ADD COLUMN IF NOT EXISTS group_comment_id text;
ALTER TABLE submission_comments ADD COLUMN IF NOT EXISTS attempt bigint;
CREATE INDEX IF NOT EXISTS idx_submission_comments_group_comment_id ON public.submission_comments USING btree (group_comment_id);

ALTER TABLE peer_reviews ADD COLUMN IF NOT EXISTS rubric_data text;
