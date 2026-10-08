-- 000168_page_view_analytics.down.sql

DROP INDEX IF EXISTS idx_page_views_user_ctx;
DROP INDEX IF EXISTS idx_page_views_ctx_created;
ALTER TABLE page_views DROP COLUMN IF EXISTS asset_category;
