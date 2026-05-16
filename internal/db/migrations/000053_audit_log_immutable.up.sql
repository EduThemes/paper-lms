-- 000053_audit_log_immutable.up.sql
--
-- Phase 13 Item 13.10 — audit_logs append-only.
--
-- State DPAs (Kansas K-12 in particular) require tamper-evident audit.
-- The audit memo flagged that audit_logs rows could be silently
-- modified by anyone with the Postgres role. A trigger that raises
-- on UPDATE / DELETE is the lightest-weight enforcement that satisfies
-- the requirement without changing application code: corrections must
-- be expressed as a new row (the "compensating entry" pattern).
--
-- (2026-05-16, 13.x.1): table name corrected from `audit_log` to
-- `audit_logs` — the table is plural (created in 000001_init.up.sql);
-- the previous draft referenced the singular form and failed migration.

BEGIN;

CREATE OR REPLACE FUNCTION reject_audit_log_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only; insert a new row to express a correction'
        USING ERRCODE = 'integrity_constraint_violation';
END $$;

DROP TRIGGER IF EXISTS audit_log_immutable ON audit_logs;

CREATE TRIGGER audit_log_immutable
BEFORE UPDATE OR DELETE ON audit_logs
FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation();

COMMIT;
