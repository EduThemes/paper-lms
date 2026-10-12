-- 000098_blueprint_content_locks.up.sql
--
-- Per-item blueprint lock settings (Canvas MasterContentTag restrictions,
-- G069). A row overrides the template's default_restrictions for one
-- piece of blueprint content; the restrictions are enforced on the
-- synced copies in every associated course.

BEGIN;

CREATE TABLE IF NOT EXISTS blueprint_content_locks (
    id bigserial PRIMARY KEY,
    blueprint_template_id bigint NOT NULL,
    content_type text NOT NULL,
    content_id bigint NOT NULL,
    restrictions jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bp_content_lock_item
    ON blueprint_content_locks USING btree (blueprint_template_id, content_type, content_id);

-- The child-side lookup (is this child row a locked blueprint copy?)
-- goes through the sync identity map by child id.
CREATE INDEX IF NOT EXISTS idx_bp_content_tag_child
    ON blueprint_content_tags USING btree (child_course_id, content_type, child_id);

COMMIT;
