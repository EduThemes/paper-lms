-- G102: custom role enforcement.
--
-- An enrollment can now carry the custom role it was granted (Canvas:
-- enrollments.role_id). The enrollment's type stays the base role
-- (TeacherEnrollment, TaEnrollment, ...); role_id names the custom role
-- whose permission set replaces the base defaults on the permission-gated
-- routes. Deleting a custom role drops enrollments back to their base role.
ALTER TABLE enrollments ADD COLUMN IF NOT EXISTS role_id bigint;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_enrollments_role_id') THEN
        ALTER TABLE enrollments
            ADD CONSTRAINT fk_enrollments_role_id
            FOREIGN KEY (role_id) REFERENCES custom_roles(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_enrollments_role_id ON enrollments USING btree (role_id);

-- Backfill: enrollments imported through SIS kept the custom role's NAME in
-- enrollments.role. Resolve it inside the course's tenant.
UPDATE enrollments e
SET role_id = cr.id
FROM courses c, custom_roles cr
WHERE e.course_id = c.id
  AND e.role_id IS NULL
  AND e.role <> e.type
  AND cr.account_id = c.account_id
  AND cr.name = e.role
  AND cr.workflow_state = 'active';

-- role_overrides: BulkUpsert relies on ON CONFLICT over
-- (role_id, permission, context_type, context_id), which needs a unique
-- index. Keep the newest row of any duplicate first.
DELETE FROM role_overrides a
USING role_overrides b
WHERE a.role_id = b.role_id
  AND a.permission = b.permission
  AND a.context_type = b.context_type
  AND a.context_id = b.context_id
  AND a.id < b.id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_role_overrides_role_perm_context
    ON role_overrides USING btree (role_id, permission, context_type, context_id);
