-- 000118_lti_advantage.down.sql
--
-- The up migration's lti_tool_configurations.privacy_level = 'public'
-- backfill is intentionally not reversed: that column predates 000118, the
-- code before it never reads it, and 'public' is the behaviour those tools
-- had before (name / email / SIS id always sent).

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
