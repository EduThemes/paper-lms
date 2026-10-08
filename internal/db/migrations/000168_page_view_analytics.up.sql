-- 000168_page_view_analytics.up.sql
--
-- Course analytics (G154). Page views are now recorded by the web client
-- (one row per course page visited, with the time spent on it) and by the
-- server for participations (a student's successful course write). The
-- analytics queries group them by course, student and day, and by the kind
-- of course content viewed (asset_category: assignments, discussions,
-- pages, ...), so:
--   * asset_category records that kind ('' when unknown);
--   * two indexes serve the per-course and per-student aggregations.

ALTER TABLE page_views ADD COLUMN IF NOT EXISTS asset_category text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_page_views_ctx_created
    ON page_views (context_type, context_id, created_at);
CREATE INDEX IF NOT EXISTS idx_page_views_user_ctx
    ON page_views (user_id, context_type, context_id);
