-- 000063_users_suspended.up.sql
--
-- Account-disable / suspend gate. Before this column the only way to lock
-- a user out was to hard-delete the row (destroying audit trail, grades,
-- enrollments) — an operational gap the red-team flagged. `suspended`
-- lets an admin disable an account: the login pipeline fails closed for a
-- suspended user across every credential path (local, SSO, passkey, MFA)
-- without touching their data.
--
-- DEFAULT FALSE so the column is a no-op for the entire existing
-- population; only an explicit admin action sets it true.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS suspended boolean NOT NULL DEFAULT false;

COMMIT;
