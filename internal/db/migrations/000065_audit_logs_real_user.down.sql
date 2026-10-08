-- 000065_audit_logs_real_user.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_audit_logs_real_user_id;

ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS real_user_id;

COMMIT;
