-- 000100_discussion_topic_parity.up.sql
--
-- Discussions lane (G085 group discussions, G087 anonymous and
-- section-specific discussions).
--
--   group_category_id    a root topic split per group in a group set
--   root_topic_id        a group's child topic points at its root
--   group_id             the group a child topic belongs to
--   anonymous_state      NULL | full_anonymity | partial_anonymity
--   is_section_specific  visible only to the sections listed in
--                        discussion_topic_section_visibilities

ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS group_category_id bigint;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS root_topic_id bigint;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS group_id bigint;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS anonymous_state text;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS is_section_specific boolean DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_discussion_topics_group_category_id ON discussion_topics(group_category_id);
CREATE INDEX IF NOT EXISTS idx_discussion_topics_root_topic_id ON discussion_topics(root_topic_id);
CREATE INDEX IF NOT EXISTS idx_discussion_topics_group_id ON discussion_topics(group_id);
-- One live child topic per (root, group): two people opening a group
-- discussion at once must not each create a copy for the same group.
CREATE UNIQUE INDEX IF NOT EXISTS idx_discussion_topics_root_group
    ON discussion_topics(root_topic_id, group_id)
    WHERE root_topic_id IS NOT NULL AND workflow_state <> 'deleted';

ALTER TABLE discussion_topics DROP CONSTRAINT IF EXISTS chk_discussion_topics_anonymous_state;
ALTER TABLE discussion_topics ADD CONSTRAINT chk_discussion_topics_anonymous_state
    CHECK (anonymous_state IS NULL OR anonymous_state IN ('full_anonymity', 'partial_anonymity'));

CREATE TABLE IF NOT EXISTS discussion_topic_section_visibilities (
    id bigserial PRIMARY KEY,
    discussion_topic_id bigint NOT NULL REFERENCES discussion_topics(id) ON DELETE CASCADE,
    course_section_id bigint NOT NULL REFERENCES course_sections(id) ON DELETE CASCADE,
    created_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dtsv_topic_section
    ON discussion_topic_section_visibilities(discussion_topic_id, course_section_id);
CREATE INDEX IF NOT EXISTS idx_discussion_topic_section_visibilities_course_section_id
    ON discussion_topic_section_visibilities(course_section_id);
