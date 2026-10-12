-- 000131_submissions_custom_grade_status_id_index.down.sql
--
-- Single statement, outside a transaction (see the up migration).

DROP INDEX CONCURRENTLY IF EXISTS idx_submissions_custom_grade_status_id;
