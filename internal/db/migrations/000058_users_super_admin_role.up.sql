-- 000058_users_super_admin_role.up.sql
--
-- Super-Admin Settings Engine, Wave 1 — role vocabulary.
--
-- Locks users.role to the values the application layer actually
-- uses, with the new 'super_admin' value joining the existing
-- Canvas-API-compatible vocabulary. The 'super_admin' role crosses
-- tenant boundaries — used by the new RequireSuperAdmin middleware
-- and special-cased by assertSameTenant so platform operators can
-- manage settings across every account.
--
-- Vocabulary justification:
--   user, admin           — the two roles the auth layer treats as
--                           privileged or not (only 'admin' gates
--                           access; everything else falls through).
--   super_admin           — new in Wave 1, owns /superadmin/*.
--   teacher, observer     — accepted by handlers.UserHandler.UpdateUserRole
--                           for Canvas-API compatibility; they don't
--                           unlock any authorization gate (only 'admin'
--                           and 'super_admin' do), so they're effectively
--                           informational. Locking them here prevents
--                           a follow-up tightening of UpdateUserRole
--                           from silently breaking existing rows.
--
-- The column previously had no CHECK constraint (init migration 000001
-- only set NOT NULL DEFAULT 'user'). This migration enforces the
-- vocabulary at the DB layer for the first time. If any pre-existing
-- row carries an unexpected value the migration fails loudly — that's
-- intentional: an unrecognized role would silently behave like 'user'
-- in auth checks, so surfacing the drift at migration time is safer.
--
-- Bootstrap of the first super_admin is wired in the setup wizard
-- (POST /setup/complete) — see internal/api/v1/handlers/setup.go.
-- Existing deployments that already have an admin keep that admin's
-- role unchanged; a follow-up CLI promote step (or an existing
-- super_admin) handles promoting a second platform operator.

BEGIN;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('user', 'admin', 'super_admin', 'teacher', 'observer'));

COMMIT;
