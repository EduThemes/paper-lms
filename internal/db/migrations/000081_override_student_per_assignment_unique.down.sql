-- 000081_override_student_per_assignment_unique.down.sql
--
-- Restoring the instance-wide UNIQUE(user_id) fails if any student now
-- has overrides on more than one assignment; such rows must be pruned
-- first.

BEGIN;

DROP INDEX IF EXISTS idx_override_assignment_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_override_user
    ON assignment_override_students USING btree (user_id);

COMMIT;
