-- 000058_users_super_admin_role.down.sql
--
-- Reverting the constraint without dropping super_admin rows would leave
-- the codebase referencing a value the column technically still permits
-- (CHECK is gone), so the rollback simply drops the constraint. Demoting
-- any existing super_admin rows back to 'admin' is an operator decision
-- and intentionally not automated here.
BEGIN;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

COMMIT;
