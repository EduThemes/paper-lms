-- 000065_audit_logs_real_user.up.sql
--
-- Masquerade attribution for the audit trail. While an admin masquerades,
-- the session's user_id is the impersonated user, so every audit row was
-- written as if the student/teacher had acted themselves. real_user_id
-- records the human who actually performed the action (the masquerading
-- admin); it is NULL for ordinary, non-masquerade requests. Mirrors
-- Canvas's real_user on page views / auditors.
--
-- Nullable, no default: existing rows predate masquerade attribution.

BEGIN;

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS real_user_id bigint;

CREATE INDEX IF NOT EXISTS idx_audit_logs_real_user_id ON audit_logs (real_user_id);

COMMIT;
