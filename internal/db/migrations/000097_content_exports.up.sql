-- 000097_content_exports.up.sql
--
-- Course content exports (Canvas ContentExport, G066/G142). One row per
-- export request; the worker fills in progress, the storage key of the
-- generated .imscc package and its size.

BEGIN;

CREATE TABLE IF NOT EXISTS content_exports (
    id bigserial PRIMARY KEY,
    course_id bigint NOT NULL,
    user_id bigint NOT NULL,
    export_type text NOT NULL DEFAULT 'common_cartridge',
    workflow_state text NOT NULL DEFAULT 'created',
    progress bigint NOT NULL DEFAULT 0,
    selection jsonb NOT NULL DEFAULT '{}',
    storage_key text,
    file_name text,
    file_size bigint NOT NULL DEFAULT 0,
    results jsonb NOT NULL DEFAULT '{}',
    error_message text,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_content_exports_course_id
    ON content_exports USING btree (course_id);

COMMIT;
