-- 000081_override_student_per_assignment_unique.up.sql
--
-- assignment_override_students carried a UNIQUE index on user_id alone
-- (idx_override_user), so a student could sit in at most ONE ad-hoc
-- override across every assignment in the instance. Canvas scopes that
-- uniqueness per assignment (a student is in at most one override of a
-- given assignment). Course pacing (B144) needs per-student overrides
-- on many assignments, which the old index made impossible.
--
-- Existing data satisfies the new index: under the old one each user had
-- at most one row in total.

BEGIN;

DROP INDEX IF EXISTS idx_override_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_override_assignment_user
    ON assignment_override_students USING btree (assignment_id, user_id);

COMMIT;
