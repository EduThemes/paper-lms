-- 000072_override_student_unique_per_assignment.up.sql
--
-- idx_override_user was UNIQUE on assignment_override_students(user_id)
-- alone, so a student could belong to only ONE assignment override in
-- the whole system; adding them to an override on a second assignment
-- failed. Canvas's constraint is one ad-hoc override per student per
-- assignment: UNIQUE (assignment_id, user_id). Existing rows cannot
-- violate the new index (user_id alone was already unique).
--
-- Deleting an override only soft-deleted it and left its student rows
-- behind, still holding the unique slot. Those orphans are removed here
-- (the service now removes them on delete).

BEGIN;

DELETE FROM assignment_override_students s
 USING assignment_overrides o
 WHERE s.assignment_override_id = o.id
   AND o.workflow_state = 'deleted';

DROP INDEX IF EXISTS idx_override_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_override_student_assignment_user
    ON assignment_override_students USING btree (assignment_id, user_id);

COMMIT;
