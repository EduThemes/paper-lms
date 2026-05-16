-- 000054_core_fks_priority.down.sql
BEGIN;

ALTER TABLE grade_change_logs DROP CONSTRAINT IF EXISTS grade_change_logs_grader_id_fkey;
ALTER TABLE grade_change_logs DROP CONSTRAINT IF EXISTS grade_change_logs_student_id_fkey;
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_user_id_fkey;
ALTER TABLE submission_comments DROP CONSTRAINT IF EXISTS submission_comments_submission_id_fkey;
ALTER TABLE assignments DROP CONSTRAINT IF EXISTS assignments_course_id_fkey;
ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_assignment_id_fkey;
ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_user_id_fkey;
ALTER TABLE enrollments DROP CONSTRAINT IF EXISTS enrollments_course_id_fkey;
ALTER TABLE enrollments DROP CONSTRAINT IF EXISTS enrollments_user_id_fkey;

COMMIT;
