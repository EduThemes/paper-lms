DROP INDEX IF EXISTS idx_role_overrides_role_perm_context;
DROP INDEX IF EXISTS idx_enrollments_role_id;
ALTER TABLE enrollments DROP CONSTRAINT IF EXISTS fk_enrollments_role_id;
ALTER TABLE enrollments DROP COLUMN IF EXISTS role_id;
