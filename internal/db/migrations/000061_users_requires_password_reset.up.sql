-- 000061_users_requires_password_reset.up.sql
--
-- Wave 1.6 follow-up — SIS / OneRoster importers generate a
-- cryptographically random initial password (PR #38) that the
-- learner has no way to recover. This column gates the login
-- pipeline so a flagged user can't mint a session until they
-- complete a "set a new password" step.
--
-- DEFAULT FALSE so the column is a no-op for the entire existing
-- population — only NEW rows the SIS / OneRoster paths create
-- after this migration get the flag set. Existing users continue
-- through the normal login path unchanged.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS requires_password_reset boolean NOT NULL DEFAULT false;

COMMIT;
