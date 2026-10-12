-- 000104_announcement_replies_sections.up.sql
--
-- G090: announcement replies (comments on an announcement that allows
-- them) and multi-section targeting (target_audience = 'sections' plus one
-- row per targeted course section).

BEGIN;

CREATE TABLE IF NOT EXISTS announcement_replies (
    id              bigserial PRIMARY KEY,
    announcement_id bigint NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    user_id         bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id       bigint REFERENCES announcement_replies(id) ON DELETE CASCADE,
    message         text   NOT NULL,
    workflow_state  text   NOT NULL DEFAULT 'active',
    created_at      timestamptz,
    updated_at      timestamptz,
    CONSTRAINT announcement_replies_workflow_state_check
        CHECK (workflow_state IN ('active', 'deleted'))
);
CREATE INDEX IF NOT EXISTS idx_announcement_replies_announcement_id
    ON announcement_replies (announcement_id);
CREATE INDEX IF NOT EXISTS idx_announcement_replies_parent_id
    ON announcement_replies (parent_id);

CREATE TABLE IF NOT EXISTS announcement_section_visibilities (
    id                bigserial PRIMARY KEY,
    announcement_id   bigint NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    course_section_id bigint NOT NULL REFERENCES course_sections(id) ON DELETE CASCADE,
    created_at        timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_announcement_section
    ON announcement_section_visibilities (announcement_id, course_section_id);
CREATE INDEX IF NOT EXISTS idx_announcement_section_visibilities_course_section_id
    ON announcement_section_visibilities (course_section_id);

COMMIT;
