-- Restoring the old per-user UNIQUE fails if a student now belongs to
-- overrides on more than one assignment; remove those rows first.
BEGIN;

DROP INDEX IF EXISTS idx_override_student_assignment_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_override_user
    ON assignment_override_students USING btree (user_id);

COMMIT;
