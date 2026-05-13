-- 000038_seed_ferpa_field_tags.down.sql
--
-- Remove only the rows seeded by the up migration. The exact (object_type,
-- field_path) tuples are scoped by the Wave 1 emit-verb inventory; hand-
-- seeded rows for other verbs (added by ops or a later migration) are
-- left in place.

BEGIN;

DELETE FROM gamification_ferpa_field_tags
WHERE (object_type, field_path) IN (
  ('Submission',      'result.score'),
  ('Submission',      'result.workflow_state'),
  ('Submission',      'context.course_id'),
  ('Submission',      'context.assignment_id'),
  ('Quiz',            'result.score'),
  ('Quiz',            'context.course_id'),
  ('Quiz',            'context.quiz_id'),
  ('Course',          'context.course_id'),
  ('Course',          'context.role'),
  ('Page',            'context.course_id'),
  ('Page',            'context.page_id'),
  ('DiscussionEntry', 'result.parent_id'),
  ('DiscussionEntry', 'context.course_id'),
  ('DiscussionEntry', 'context.discussion_topic_id'),
  ('Outcome',         'result.score'),
  ('Outcome',         'result.percent'),
  ('Outcome',         'result.mastery'),
  ('Outcome',         'result.result_id'),
  ('Outcome',         'context.context_type'),
  ('Outcome',         'context.context_id'),
  ('Outcome',         'context.associated_asset_type'),
  ('Outcome',         'context.associated_asset_id'),
  ('Outcome',         'context.calculation_method'),
  ('Rubric',          'result.score'),
  ('Rubric',          'result.data'),
  ('Rubric',          'result.assessment_type'),
  ('Rubric',          'result.assessment_id'),
  ('Rubric',          'result.assessor_id'),
  ('Rubric',          'context.rubric_id'),
  ('Rubric',          'context.rubric_association_id'),
  ('Rubric',          'context.context_type'),
  ('Rubric',          'context.context_id')
);

COMMIT;
