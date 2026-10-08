-- 000082_blueprint_content_tags.up.sql
--
-- Blueprint sync identity map (Canvas MasterContentTag/ChildContentTag).
-- One row per (template, child course, content type, source id) records
-- which child row is the synced copy of a blueprint item and when it was
-- last synced. Sync previously matched copies by title and skipped any
-- match, so edits in the blueprint never reached child courses (B041).

BEGIN;

CREATE TABLE IF NOT EXISTS blueprint_content_tags (
    id bigserial PRIMARY KEY,
    blueprint_template_id bigint NOT NULL,
    child_course_id bigint NOT NULL,
    content_type text NOT NULL,
    source_id bigint NOT NULL,
    child_id bigint NOT NULL,
    synced_at timestamptz NOT NULL,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bp_content_tag_source
    ON blueprint_content_tags USING btree (blueprint_template_id, child_course_id, content_type, source_id);

COMMIT;
