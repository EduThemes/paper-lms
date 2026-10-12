-- 000130_submissions_anonymous_id_index.down.sql
--
-- Single statement, outside a transaction (see the up migration).

DROP INDEX CONCURRENTLY IF EXISTS idx_submissions_anonymous_id;
