-- 000072_override_student_unique_per_assignment.up.sql
--
-- idx_override_user was UNIQUE on assignment_override_students(user_id)
-- alone, so a student could belong to only ONE assignment override in
-- the whole system; adding them to an override on a second assignment
-- failed. Canvas's constraint is one ad-hoc override per student per
-- assignment: UNIQUE (assignment_id, user_id). Existing rows cannot
-- violate the new index (user_id alone was already unique).

BEGIN;

DROP INDEX IF EXISTS idx_override_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_override_student_assignment_user
    ON assignment_override_students USING btree (assignment_id, user_id);

COMMIT;
