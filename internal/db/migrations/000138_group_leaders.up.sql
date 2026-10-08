-- 000138_group_leaders.up.sql
--
-- G100: group leaders. A group has at most one leader, who must be one of
-- its members. The leader is set by an instructor, or picked automatically
-- (first member / random member) when the group set has auto_leader set.
-- Deleting the user clears the leadership rather than the group.

ALTER TABLE groups ADD COLUMN IF NOT EXISTS leader_id bigint REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_groups_leader_id ON groups (leader_id);
