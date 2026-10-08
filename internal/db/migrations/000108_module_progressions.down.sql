-- 000108_module_progressions.down.sql

BEGIN;

DROP TABLE IF EXISTS module_item_user_actions;
DROP TABLE IF EXISTS context_module_progressions;
ALTER TABLE context_modules DROP COLUMN IF EXISTS requirement_count;

COMMIT;
