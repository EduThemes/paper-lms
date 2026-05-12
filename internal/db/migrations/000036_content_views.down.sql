-- Reverse of 000036. Drops the content-view aggregate table. Row data is LOST.

BEGIN;

DROP INDEX IF EXISTS idx_content_views_user_last;
DROP INDEX IF EXISTS idx_content_views_user_object;

DROP TABLE IF EXISTS content_views;

COMMIT;
