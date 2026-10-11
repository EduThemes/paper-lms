-- 000098_blueprint_content_locks.down.sql
BEGIN;
DROP INDEX IF EXISTS idx_bp_content_tag_child;
DROP TABLE IF EXISTS blueprint_content_locks;
COMMIT;
