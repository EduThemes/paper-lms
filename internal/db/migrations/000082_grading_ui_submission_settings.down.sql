-- 000082_grading_ui_submission_settings.down.sql
ALTER TABLE peer_reviews DROP COLUMN IF EXISTS rubric_data;

DROP INDEX IF EXISTS idx_submission_comments_group_comment_id;
ALTER TABLE submission_comments DROP COLUMN IF EXISTS attempt;
ALTER TABLE submission_comments DROP COLUMN IF EXISTS group_comment_id;
ALTER TABLE submission_comments DROP COLUMN IF EXISTS attachments;

DROP INDEX IF EXISTS idx_submissions_custom_grade_status_id;
DROP INDEX IF EXISTS idx_submissions_anonymous_id;
ALTER TABLE submissions DROP COLUMN IF EXISTS custom_grade_status_id;
ALTER TABLE submissions DROP COLUMN IF EXISTS extra_attempts;
ALTER TABLE submissions DROP COLUMN IF EXISTS anonymous_id;

DROP INDEX IF EXISTS idx_assignments_annotatable_attachment_id;
ALTER TABLE assignments DROP COLUMN IF EXISTS peer_reviews_due_at;
ALTER TABLE assignments DROP COLUMN IF EXISTS anonymous_peer_reviews;
ALTER TABLE assignments DROP COLUMN IF EXISTS grades_published_at;
ALTER TABLE assignments DROP COLUMN IF EXISTS grader_names_visible_to_final_grader;
ALTER TABLE assignments DROP COLUMN IF EXISTS graders_anonymous_to_graders;
ALTER TABLE assignments DROP COLUMN IF EXISTS grader_comments_visible_to_graders;
ALTER TABLE assignments DROP COLUMN IF EXISTS final_grader_id;
ALTER TABLE assignments DROP COLUMN IF EXISTS grader_count;
ALTER TABLE assignments DROP COLUMN IF EXISTS moderated_grading;
ALTER TABLE assignments DROP COLUMN IF EXISTS grade_group_students_individually;
ALTER TABLE assignments DROP COLUMN IF EXISTS annotatable_attachment_id;
ALTER TABLE assignments DROP COLUMN IF EXISTS allowed_extensions;
ALTER TABLE assignments DROP COLUMN IF EXISTS allowed_attempts;
