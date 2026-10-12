-- 000132_page_views_ctx_created_index.up.sql
--
-- Per-course page-view aggregation index for course analytics (G154,
-- see 000125), built concurrently: page_views is the largest table.
--
-- CREATE INDEX CONCURRENTLY cannot run inside a transaction block, so this
-- file holds exactly one statement: golang-migrate sends the file as one
-- simple-protocol query, which Postgres runs outside a transaction when it
-- contains a single statement. Keep it that way (no BEGIN/COMMIT, no second
-- statement), or Postgres rejects CONCURRENTLY and the migration fails.
-- If the build is interrupted it can leave an INVALID index behind, which
-- IF NOT EXISTS would then skip: drop it (DROP INDEX CONCURRENTLY) and
-- re-run the migration.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_page_views_ctx_created
    ON page_views (context_type, context_id, created_at);
