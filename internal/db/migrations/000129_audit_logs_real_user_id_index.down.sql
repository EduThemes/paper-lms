-- 000129_audit_logs_real_user_id_index.down.sql
--
-- Single statement, outside a transaction (see the up migration).

DROP INDEX CONCURRENTLY IF EXISTS idx_audit_logs_real_user_id;
