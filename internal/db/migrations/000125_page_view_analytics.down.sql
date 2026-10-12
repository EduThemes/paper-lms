-- 000125_page_view_analytics.down.sql
--
-- The indexes now come from 000132 / 000133 (whose down migrations drop
-- them first); the DROPs stay for databases built before that split.

DROP INDEX IF EXISTS idx_page_views_user_ctx;
DROP INDEX IF EXISTS idx_page_views_ctx_created;
ALTER TABLE page_views DROP COLUMN IF EXISTS asset_category;
