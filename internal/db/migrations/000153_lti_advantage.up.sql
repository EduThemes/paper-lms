-- 000153_lti_advantage.up.sql
--
-- LTI 1.3 / Advantage parity lane (G125, G128, G130, G131, G133).
--
-- lti_resource_links gains the placement a link was created from and the
-- Paper object it is attached to: an assignment (assignment_selection /
-- external_tool submission type) or a module item (ContextExternalTool
-- content tag). A course-level link (editor_button, link_selection) has
-- neither.
--
-- lti_line_items gains developer_key_id, the tool that owns the line item
-- (AGS: a tool sees and edits only its own line items), and the AGS
-- startDateTime / endDateTime fields.
--
-- context_external_tools gains privacy_level, used by LTI 1.1 installs
-- (LTI 1.3 tools take it from their tool configuration).

BEGIN;

ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS assignment_id bigint;
ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS content_tag_id bigint;
ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS placement text;
CREATE INDEX IF NOT EXISTS idx_lti_resource_links_assignment_id ON public.lti_resource_links USING btree (assignment_id);
CREATE INDEX IF NOT EXISTS idx_lti_resource_links_content_tag_id ON public.lti_resource_links USING btree (content_tag_id);

ALTER TABLE lti_line_items ADD COLUMN IF NOT EXISTS developer_key_id bigint;
ALTER TABLE lti_line_items ADD COLUMN IF NOT EXISTS start_date_time timestamptz;
ALTER TABLE lti_line_items ADD COLUMN IF NOT EXISTS end_date_time timestamptz;
CREATE INDEX IF NOT EXISTS idx_lti_line_items_developer_key_id ON public.lti_line_items USING btree (developer_key_id);

ALTER TABLE context_external_tools ADD COLUMN IF NOT EXISTS privacy_level text DEFAULT 'anonymous';

COMMIT;
