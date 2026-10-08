BEGIN;

ALTER TABLE context_external_tools DROP COLUMN IF EXISTS privacy_level;

DROP INDEX IF EXISTS idx_lti_line_items_developer_key_id;
ALTER TABLE lti_line_items DROP COLUMN IF EXISTS end_date_time;
ALTER TABLE lti_line_items DROP COLUMN IF EXISTS start_date_time;
ALTER TABLE lti_line_items DROP COLUMN IF EXISTS developer_key_id;

DROP INDEX IF EXISTS idx_lti_resource_links_content_tag_id;
DROP INDEX IF EXISTS idx_lti_resource_links_assignment_id;
ALTER TABLE lti_resource_links DROP COLUMN IF EXISTS placement;
ALTER TABLE lti_resource_links DROP COLUMN IF EXISTS content_tag_id;
ALTER TABLE lti_resource_links DROP COLUMN IF EXISTS assignment_id;

COMMIT;
