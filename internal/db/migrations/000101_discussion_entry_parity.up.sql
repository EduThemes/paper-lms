-- 000101_discussion_entry_parity.up.sql
--
-- Discussions lane (G086 reply attachments, G087 anonymous authors,
-- G088 @mentions).
--
--   discussion_entries.attachment_id            a file attached to the post
--   discussion_entries.is_anonymous_author      partial anonymity, per post
--   discussion_topic_participants.anonymous_id  a participant's alias
--   discussion_entry_mentions                   who a post @mentioned, so an
--                                               edit only notifies new names

ALTER TABLE discussion_entries ADD COLUMN IF NOT EXISTS attachment_id bigint;
ALTER TABLE discussion_entries ADD COLUMN IF NOT EXISTS is_anonymous_author boolean DEFAULT false;

ALTER TABLE discussion_topic_participants ADD COLUMN IF NOT EXISTS anonymous_id text;

CREATE TABLE IF NOT EXISTS discussion_entry_mentions (
    id bigserial PRIMARY KEY,
    discussion_entry_id bigint NOT NULL REFERENCES discussion_entries(id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    created_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dem_entry_user
    ON discussion_entry_mentions(discussion_entry_id, user_id);
CREATE INDEX IF NOT EXISTS idx_discussion_entry_mentions_user_id
    ON discussion_entry_mentions(user_id);
