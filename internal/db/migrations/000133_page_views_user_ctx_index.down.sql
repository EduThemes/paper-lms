-- 000133_page_views_user_ctx_index.down.sql
--
-- Single statement, outside a transaction (see the up migration).

DROP INDEX CONCURRENTLY IF EXISTS idx_page_views_user_ctx;
