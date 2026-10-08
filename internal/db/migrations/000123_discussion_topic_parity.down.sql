-- 000123_discussion_topic_parity.down.sql
DROP TABLE IF EXISTS discussion_topic_section_visibilities;

ALTER TABLE discussion_topics DROP CONSTRAINT IF EXISTS chk_discussion_topics_anonymous_state;
DROP INDEX IF EXISTS idx_discussion_topics_root_group;
DROP INDEX IF EXISTS idx_discussion_topics_group_id;
DROP INDEX IF EXISTS idx_discussion_topics_root_topic_id;
DROP INDEX IF EXISTS idx_discussion_topics_group_category_id;

ALTER TABLE discussion_topics DROP COLUMN IF EXISTS is_section_specific;
ALTER TABLE discussion_topics DROP COLUMN IF EXISTS anonymous_state;
ALTER TABLE discussion_topics DROP COLUMN IF EXISTS group_id;
ALTER TABLE discussion_topics DROP COLUMN IF EXISTS root_topic_id;
ALTER TABLE discussion_topics DROP COLUMN IF EXISTS group_category_id;
