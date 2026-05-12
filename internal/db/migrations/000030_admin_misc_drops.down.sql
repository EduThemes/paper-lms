-- Reverse of 000030: re-add the dropped columns with their original types
-- and defaults from migration 000001. Structural only — the row data is LOST.
--
-- IMPORTANT: rows that existed when 000030 ran no longer have legacy-column
-- values. After this down runs, all legacy columns are NULL/default. If you
-- need the legacy data back, you must restore from a backup taken before
-- 000030 ran; the .up.sql is data-destructive by design.
--
-- Some NOT NULL columns from 000001 are recreated as nullable here because
-- re-adding NOT NULL on a populated table would fail (rows would have NULL).
-- Operators rolling back and restoring data can re-tighten the constraint
-- separately if needed.

BEGIN;

-- ============================================================
-- role_overrides
-- ============================================================
ALTER TABLE role_overrides ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE role_overrides ADD COLUMN IF NOT EXISTS custom_role_id bigint;

-- ============================================================
-- custom_roles
-- ============================================================
ALTER TABLE custom_roles ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================
-- pii_access_logs
-- ============================================================
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS user_id bigint;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS accessed_by bigint;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS data_accessed text;
ALTER TABLE pii_access_logs ADD COLUMN IF NOT EXISTS purpose text;

-- ============================================================
-- grade_change_logs
-- ============================================================
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS excused_before boolean DEFAULT false;
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS excused_after boolean DEFAULT false;
ALTER TABLE grade_change_logs ADD COLUMN IF NOT EXISTS graded_anonymously boolean DEFAULT false;

-- ============================================================
-- audit_logs
-- ============================================================
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS data text;

-- ============================================================
-- content_migrations
-- ============================================================
ALTER TABLE content_migrations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================
-- one_roster_sync_logs
-- ============================================================
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'running';
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS finished_at timestamptz;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS created_count bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS updated_count bigint DEFAULT 0;
ALTER TABLE one_roster_sync_logs ADD COLUMN IF NOT EXISTS error_count bigint DEFAULT 0;

-- ============================================================
-- one_roster_connections
-- ============================================================
ALTER TABLE one_roster_connections ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================
-- sis_batch_errors
-- ============================================================
ALTER TABLE sis_batch_errors ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE sis_batch_errors ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE sis_batch_errors ADD COLUMN IF NOT EXISTS file_name text;
ALTER TABLE sis_batch_errors ADD COLUMN IF NOT EXISTS row_number bigint;

-- ============================================================
-- sis_batches
-- ============================================================
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS batch_mode boolean DEFAULT false;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS started_at timestamptz;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS ended_at timestamptz;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS diffing_data_set_identifier text;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS created_count bigint DEFAULT 0;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS updated_count bigint DEFAULT 0;
ALTER TABLE sis_batches ADD COLUMN IF NOT EXISTS deleted_count bigint DEFAULT 0;

-- ============================================================
-- blueprint_migrations
-- ============================================================
ALTER TABLE blueprint_migrations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE blueprint_migrations ADD COLUMN IF NOT EXISTS imports_status text;
ALTER TABLE blueprint_migrations ADD COLUMN IF NOT EXISTS started_at timestamptz;

-- ============================================================
-- blueprint_subscriptions
-- ============================================================
ALTER TABLE blueprint_subscriptions ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================
-- blueprint_templates
-- ============================================================
ALTER TABLE blueprint_templates ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE blueprint_templates ADD COLUMN IF NOT EXISTS restrictions_by_type text;
ALTER TABLE blueprint_templates ADD COLUMN IF NOT EXISTS use_default_restrictions_by_type boolean DEFAULT false;

-- ============================================================
-- group_memberships
-- ============================================================
ALTER TABLE group_memberships ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================
-- groups
-- ============================================================
ALTER TABLE groups ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================
-- group_categories
-- ============================================================
ALTER TABLE group_categories ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE group_categories ADD COLUMN IF NOT EXISTS context_type text;
ALTER TABLE group_categories ADD COLUMN IF NOT EXISTS context_id bigint;

-- ============================================================
-- portfolio_comments
-- ============================================================
ALTER TABLE portfolio_comments ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE portfolio_comments ADD COLUMN IF NOT EXISTS comment text;

-- ============================================================
-- portfolio_templates
-- ============================================================
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS title text;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS structure text;
ALTER TABLE portfolio_templates ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'active';

-- ============================================================
-- portfolio_reflections
-- ============================================================
ALTER TABLE portfolio_reflections ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE portfolio_reflections ADD COLUMN IF NOT EXISTS portfolio_artifact_id bigint;
ALTER TABLE portfolio_reflections ADD COLUMN IF NOT EXISTS reflection_type text DEFAULT 'text';
ALTER TABLE portfolio_reflections ADD COLUMN IF NOT EXISTS metadata text;

-- ============================================================
-- portfolio_artifacts
-- ============================================================
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS portfolio_section_id bigint;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS submission_id bigint;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS attachment_id bigint;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS content text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS url text;
ALTER TABLE portfolio_artifacts ADD COLUMN IF NOT EXISTS metadata text;

-- ============================================================
-- portfolio_sections
-- ============================================================
ALTER TABLE portfolio_sections ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE portfolio_sections ADD COLUMN IF NOT EXISTS description text;

-- ============================================================
-- portfolios
-- ============================================================
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS visibility text DEFAULT 'private';
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS template_id bigint;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS published_at timestamptz;
ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS share_token text;

-- ============================================================
-- data_export_requests
-- ============================================================
ALTER TABLE data_export_requests ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE data_export_requests ADD COLUMN IF NOT EXISTS requested_by bigint;
ALTER TABLE data_export_requests ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'pending';

-- ============================================================
-- data_deletion_requests
-- ============================================================
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS requested_by bigint;
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'pending';
ALTER TABLE data_deletion_requests ADD COLUMN IF NOT EXISTS data_types text;

-- ============================================================
-- data_retention_policies
-- ============================================================
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS data_type text;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS retention_period_days bigint;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS action_after_retention text DEFAULT 'anonymize';
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS last_applied_at timestamptz;
ALTER TABLE data_retention_policies ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'active';

-- ============================================================
-- age_verifications
-- ============================================================
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS is_under_13 boolean DEFAULT false;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS ip_address text;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS verification_method text;
ALTER TABLE age_verifications ADD COLUMN IF NOT EXISTS verified_at timestamptz;

-- ============================================================
-- data_processing_agreements
-- ============================================================
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS effective_date timestamptz;
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS expiration_date timestamptz;
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS signed_by bigint;
ALTER TABLE data_processing_agreements ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'active';

-- ============================================================
-- parental_consents
-- ============================================================
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS granted_at timestamptz;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS verification_code text;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS verification_expires_at timestamptz;
ALTER TABLE parental_consents ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'pending';

-- ============================================================
-- student_accommodations
-- ============================================================
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS details text;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS extended_time_multiplier double precision DEFAULT 1.0;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'active';
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS approved_by bigint;
ALTER TABLE student_accommodations ADD COLUMN IF NOT EXISTS expires_at timestamptz;

COMMIT;
