-- 000108_group_leaders.down.sql

DROP INDEX IF EXISTS idx_groups_leader_id;

ALTER TABLE groups DROP COLUMN IF EXISTS leader_id;
