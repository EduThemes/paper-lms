-- 000038_seed_ferpa_field_tags.up.sql
--
-- Phase 6 Wave 1 Sprint D-3 — seed gamification_ferpa_field_tags with
-- the policy classifications for every result/context field shape the
-- seven live emit verbs produce (graded submission, completed quiz,
-- enrolled course, viewed page, posted discussion entry, mastered
-- outcome, assessed rubric).
--
-- Without these rows the FERPA guard has no rules to enforce — it
-- silently passes every event regardless of classification. With them,
-- the emitter's policy-flag-derivation step appends the required
-- 'ferpa_protected' + 'education_record' flags whenever an
-- education-record-tagged field is present, and the guard verifies
-- those flags are set before the event is persisted.
--
-- Classification policy:
--   education_record    — graded evaluation: score, percent, mastery,
--                          per-criterion rubric ratings. FERPA-protected.
--   directory_information — course/section enrollment refs, role,
--                          assessor identity. Treatable as directory
--                          info under FERPA absent an opt-out.
--   instructor_metadata  — workflow_state, assessment_type, calc method.
--                          Visible to instructors, not students or public.
--   non_PII              — internal references and topic identifiers
--                          that carry no personal data on their own.
--
-- Idempotent via ON CONFLICT — re-running won't fail if a partial set
-- has been hand-seeded.

BEGIN;

INSERT INTO gamification_ferpa_field_tags (object_type, field_path, classification, description) VALUES
  ('Submission',      'result.score',                  'education_record',     'Graded score is a FERPA-protected education record.'),
  ('Submission',      'result.workflow_state',         'instructor_metadata',  'Graded / submitted / pending — instructor-side state.'),
  ('Submission',      'context.course_id',             'directory_information','Course enrollment reference.'),
  ('Submission',      'context.assignment_id',         'directory_information','Course assignment reference.'),

  ('Quiz',            'result.score',                  'education_record',     'Graded quiz score is FERPA-protected.'),
  ('Quiz',            'context.course_id',             'directory_information','Course enrollment reference.'),
  ('Quiz',            'context.quiz_id',               'directory_information','Quiz reference within a course.'),

  ('Course',          'context.course_id',             'directory_information','Course enrollment reference.'),
  ('Course',          'context.role',                  'directory_information','Enrollment role (Student/Teacher/etc).'),

  ('Page',            'context.course_id',             'directory_information','Course enrollment reference.'),
  ('Page',            'context.page_id',               'non_PII',              'Page identifier alone is not PII.'),

  ('DiscussionEntry', 'result.parent_id',              'non_PII',              'Reply-parent identifier is not PII.'),
  ('DiscussionEntry', 'context.course_id',             'directory_information','Course enrollment reference.'),
  ('DiscussionEntry', 'context.discussion_topic_id',   'non_PII',              'Discussion topic identifier is not PII.'),

  ('Outcome',         'result.score',                  'education_record',     'Outcome score is a FERPA-protected education record.'),
  ('Outcome',         'result.percent',                'education_record',     'Outcome percent is FERPA-protected.'),
  ('Outcome',         'result.mastery',                'education_record',     'Outcome mastery flag is FERPA-protected.'),
  ('Outcome',         'result.result_id',              'non_PII',              'Internal result row reference.'),
  ('Outcome',         'context.context_type',          'non_PII',              'Outcome context scope type (Course/Account).'),
  ('Outcome',         'context.context_id',            'directory_information','Course or account scope reference.'),
  ('Outcome',         'context.associated_asset_type', 'non_PII',              'Linked asset type (Assignment/Quiz).'),
  ('Outcome',         'context.associated_asset_id',   'non_PII',              'Linked asset reference.'),
  ('Outcome',         'context.calculation_method',    'instructor_metadata',  'Mastery calc method — instructor configuration.'),

  ('Rubric',          'result.score',                  'education_record',     'Rubric overall score is FERPA-protected.'),
  ('Rubric',          'result.data',                   'education_record',     'Per-criterion ratings are FERPA-protected.'),
  ('Rubric',          'result.assessment_type',        'instructor_metadata',  'grading vs peer_review.'),
  ('Rubric',          'result.assessment_id',          'non_PII',              'Internal assessment reference.'),
  ('Rubric',          'result.assessor_id',            'directory_information','Assessor identity — directory info.'),
  ('Rubric',          'context.rubric_id',             'non_PII',              'Rubric reference.'),
  ('Rubric',          'context.rubric_association_id', 'non_PII',              'Internal association reference.'),
  ('Rubric',          'context.context_type',          'non_PII',              'Rubric context scope type.'),
  ('Rubric',          'context.context_id',            'directory_information','Course or account scope reference.')
ON CONFLICT (object_type, field_path) DO NOTHING;

COMMIT;
