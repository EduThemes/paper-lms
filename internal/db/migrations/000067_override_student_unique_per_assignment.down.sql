-- Restoring the old per-user UNIQUE requires each student to be in at
-- most one override row. Rows beyond the oldest per student (overrides on
-- other assignments, allowed since 000067) are dropped first; otherwise
-- the CREATE UNIQUE INDEX would fail.
BEGIN;

DELETE FROM assignment_override_students s
 USING assignment_override_students keep
 WHERE s.user_id = keep.user_id
   AND s.id > keep.id;

DROP INDEX IF EXISTS idx_override_student_assignment_user;

CREATE UNIQUE INDEX IF NOT EXISTS idx_override_user
    ON assignment_override_students USING btree (user_id);

COMMIT;
