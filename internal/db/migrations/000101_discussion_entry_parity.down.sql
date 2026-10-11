-- 000101_discussion_entry_parity.down.sql
DROP TABLE IF EXISTS discussion_entry_mentions;

ALTER TABLE discussion_topic_participants DROP COLUMN IF EXISTS anonymous_id;

ALTER TABLE discussion_entries DROP COLUMN IF EXISTS is_anonymous_author;
ALTER TABLE discussion_entries DROP COLUMN IF EXISTS attachment_id;
