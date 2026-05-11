-- Performance indexes identified by perf audit.
-- Table and column names verified against internal/domain/models/.

CREATE INDEX IF NOT EXISTS idx_submissions_assignment_user ON submissions(assignment_id, user_id);
CREATE INDEX IF NOT EXISTS idx_submissions_user_state ON submissions(user_id, workflow_state);
CREATE INDEX IF NOT EXISTS idx_assignments_course_state ON assignments(course_id, workflow_state) WHERE workflow_state <> 'deleted';
CREATE INDEX IF NOT EXISTS idx_modules_course_position ON context_modules(course_id, position) WHERE workflow_state <> 'deleted';
CREATE INDEX IF NOT EXISTS idx_content_tags_module_position ON content_tags(context_module_id, position);
CREATE INDEX IF NOT EXISTS idx_discussion_entries_topic_created ON discussion_entries(discussion_topic_id, created_at);
CREATE INDEX IF NOT EXISTS idx_quiz_submissions_quiz_user ON quiz_submissions(quiz_id, user_id);
CREATE INDEX IF NOT EXISTS idx_submission_comments_sub_created ON submission_comments(submission_id, created_at);
CREATE INDEX IF NOT EXISTS idx_assignment_overrides_assignment ON assignment_overrides(assignment_id);
CREATE INDEX IF NOT EXISTS idx_assignment_override_students_override ON assignment_override_students(assignment_override_id);
CREATE INDEX IF NOT EXISTS idx_module_prerequisites_module ON module_prerequisites(module_id);
CREATE INDEX IF NOT EXISTS idx_outcome_alignments_assignment ON outcome_alignments(assignment_id);
CREATE INDEX IF NOT EXISTS idx_outcome_alignments_course ON outcome_alignments(course_id);
CREATE INDEX IF NOT EXISTS idx_peer_reviews_assignment_reviewer ON peer_reviews(assignment_id, reviewer_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_course_created ON audit_logs(course_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_grade_change_logs_course_created ON grade_change_logs(course_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
