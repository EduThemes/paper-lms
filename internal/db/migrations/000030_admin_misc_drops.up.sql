-- Wave 2c: drop legacy columns from admin + compliance + portfolios + misc domain.
-- DATA-DESTRUCTIVE.
--
-- Deprecation window: added 2026-05-11. Operators with production data that
-- predates this migration MUST have run 000023 first; that migration copies
-- legacy data into the new GORM-model columns. Once this migration runs the
-- dropped columns are gone — the .down.sql can recreate the shape but not
-- the data.
--
-- Columns dropped fall into three categories:
--
--   1. Wave 2b sources (data already copied to new columns in 000023):
--        age_verifications.is_under_13           → is_under13
--        portfolio_artifacts.portfolio_section_id → section_id
--        portfolio_artifacts.attachment_id        → (source_type, source_id)
--        portfolio_artifacts.submission_id        → (source_type, source_id)
--        role_overrides.custom_role_id            → role_id
--
--   2. SOFT_DELETE_LEFTOVER: deleted_at — the model removed gorm.DeletedAt.
--
--   3. UNKNOWN / removed-feature columns confirmed zero live references in
--      non-test Go code outside the migrations directory.
--
-- Tables with no stale columns (skipped): feature_flags, content_embeddings,
-- shared_content, shared_content_favorites.

BEGIN;

-- ============================================================
-- student_accommodations (6 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE student_accommodations DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.StudentAccommodation
ALTER TABLE student_accommodations DROP COLUMN IF EXISTS details;
ALTER TABLE student_accommodations DROP COLUMN IF EXISTS extended_time_multiplier;
ALTER TABLE student_accommodations DROP COLUMN IF EXISTS workflow_state;
ALTER TABLE student_accommodations DROP COLUMN IF EXISTS approved_by;
ALTER TABLE student_accommodations DROP COLUMN IF EXISTS expires_at;

-- ============================================================
-- parental_consents (5 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE parental_consents DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.ParentalConsent
ALTER TABLE parental_consents DROP COLUMN IF EXISTS granted_at;
ALTER TABLE parental_consents DROP COLUMN IF EXISTS verification_code;
ALTER TABLE parental_consents DROP COLUMN IF EXISTS verification_expires_at;
ALTER TABLE parental_consents DROP COLUMN IF EXISTS workflow_state;

-- ============================================================
-- data_processing_agreements (5 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE data_processing_agreements DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.DataProcessingAgreement
ALTER TABLE data_processing_agreements DROP COLUMN IF EXISTS effective_date;
ALTER TABLE data_processing_agreements DROP COLUMN IF EXISTS expiration_date;
ALTER TABLE data_processing_agreements DROP COLUMN IF EXISTS signed_by;
ALTER TABLE data_processing_agreements DROP COLUMN IF EXISTS workflow_state;

-- ============================================================
-- age_verifications (5 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE age_verifications DROP COLUMN IF EXISTS deleted_at;
-- Wave 2b source (data copied to is_under13 in 000023)
ALTER TABLE age_verifications DROP COLUMN IF EXISTS is_under_13;
-- UNKNOWN — no matching field in models.AgeVerification
ALTER TABLE age_verifications DROP COLUMN IF EXISTS ip_address;
ALTER TABLE age_verifications DROP COLUMN IF EXISTS verification_method;
ALTER TABLE age_verifications DROP COLUMN IF EXISTS verified_at;

-- ============================================================
-- data_retention_policies (6 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE data_retention_policies DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.DataRetentionPolicy
ALTER TABLE data_retention_policies DROP COLUMN IF EXISTS action_after_retention;
ALTER TABLE data_retention_policies DROP COLUMN IF EXISTS data_type;
ALTER TABLE data_retention_policies DROP COLUMN IF EXISTS last_applied_at;
ALTER TABLE data_retention_policies DROP COLUMN IF EXISTS retention_period_days;
ALTER TABLE data_retention_policies DROP COLUMN IF EXISTS workflow_state;

-- ============================================================
-- data_deletion_requests (4 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE data_deletion_requests DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.DataDeletionRequest
ALTER TABLE data_deletion_requests DROP COLUMN IF EXISTS data_types;
ALTER TABLE data_deletion_requests DROP COLUMN IF EXISTS requested_by;
ALTER TABLE data_deletion_requests DROP COLUMN IF EXISTS workflow_state;

-- ============================================================
-- data_export_requests (3 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE data_export_requests DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.DataExportRequest
ALTER TABLE data_export_requests DROP COLUMN IF EXISTS requested_by;
ALTER TABLE data_export_requests DROP COLUMN IF EXISTS workflow_state;

-- ============================================================
-- portfolios (5 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE portfolios DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.Portfolio
ALTER TABLE portfolios DROP COLUMN IF EXISTS published_at;
ALTER TABLE portfolios DROP COLUMN IF EXISTS share_token;
ALTER TABLE portfolios DROP COLUMN IF EXISTS template_id;
ALTER TABLE portfolios DROP COLUMN IF EXISTS visibility;

-- ============================================================
-- portfolio_sections (2 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE portfolio_sections DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.PortfolioSection
ALTER TABLE portfolio_sections DROP COLUMN IF EXISTS description;

-- ============================================================
-- portfolio_artifacts (7 cols)
-- ============================================================
-- Wave 2b sources (data copied in 000023)
ALTER TABLE portfolio_artifacts DROP COLUMN IF EXISTS portfolio_section_id;
ALTER TABLE portfolio_artifacts DROP COLUMN IF EXISTS attachment_id;
ALTER TABLE portfolio_artifacts DROP COLUMN IF EXISTS submission_id;
-- SOFT_DELETE_LEFTOVER
ALTER TABLE portfolio_artifacts DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.PortfolioArtifact
ALTER TABLE portfolio_artifacts DROP COLUMN IF EXISTS content;
ALTER TABLE portfolio_artifacts DROP COLUMN IF EXISTS metadata;
ALTER TABLE portfolio_artifacts DROP COLUMN IF EXISTS url;

-- ============================================================
-- portfolio_reflections (4 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE portfolio_reflections DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.PortfolioReflection
ALTER TABLE portfolio_reflections DROP COLUMN IF EXISTS metadata;
ALTER TABLE portfolio_reflections DROP COLUMN IF EXISTS portfolio_artifact_id;
ALTER TABLE portfolio_reflections DROP COLUMN IF EXISTS reflection_type;

-- ============================================================
-- portfolio_templates (4 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE portfolio_templates DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.PortfolioTemplate
ALTER TABLE portfolio_templates DROP COLUMN IF EXISTS structure;
ALTER TABLE portfolio_templates DROP COLUMN IF EXISTS title;
ALTER TABLE portfolio_templates DROP COLUMN IF EXISTS workflow_state;

-- ============================================================
-- portfolio_comments (2 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE portfolio_comments DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.PortfolioComment
ALTER TABLE portfolio_comments DROP COLUMN IF EXISTS comment;

-- ============================================================
-- group_categories (3 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE group_categories DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — replaced by course_id / account_id in models.GroupCategory
ALTER TABLE group_categories DROP COLUMN IF EXISTS context_id;
ALTER TABLE group_categories DROP COLUMN IF EXISTS context_type;

-- ============================================================
-- groups (1 col)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE groups DROP COLUMN IF EXISTS deleted_at;

-- ============================================================
-- group_memberships (1 col)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE group_memberships DROP COLUMN IF EXISTS deleted_at;

-- ============================================================
-- blueprint_templates (3 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE blueprint_templates DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.BlueprintTemplate
ALTER TABLE blueprint_templates DROP COLUMN IF EXISTS restrictions_by_type;
ALTER TABLE blueprint_templates DROP COLUMN IF EXISTS use_default_restrictions_by_type;

-- ============================================================
-- blueprint_subscriptions (1 col)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE blueprint_subscriptions DROP COLUMN IF EXISTS deleted_at;

-- ============================================================
-- blueprint_migrations (3 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE blueprint_migrations DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.BlueprintMigration
ALTER TABLE blueprint_migrations DROP COLUMN IF EXISTS imports_status;
ALTER TABLE blueprint_migrations DROP COLUMN IF EXISTS started_at;

-- ============================================================
-- sis_batches (8 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE sis_batches DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.SISBatch
ALTER TABLE sis_batches DROP COLUMN IF EXISTS batch_mode;
ALTER TABLE sis_batches DROP COLUMN IF EXISTS created_count;
ALTER TABLE sis_batches DROP COLUMN IF EXISTS deleted_count;
ALTER TABLE sis_batches DROP COLUMN IF EXISTS diffing_data_set_identifier;
ALTER TABLE sis_batches DROP COLUMN IF EXISTS ended_at;
ALTER TABLE sis_batches DROP COLUMN IF EXISTS started_at;
ALTER TABLE sis_batches DROP COLUMN IF EXISTS updated_count;

-- ============================================================
-- sis_batch_errors (4 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE sis_batch_errors DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.SISBatchError
ALTER TABLE sis_batch_errors DROP COLUMN IF EXISTS file_name;
ALTER TABLE sis_batch_errors DROP COLUMN IF EXISTS row_number;
ALTER TABLE sis_batch_errors DROP COLUMN IF EXISTS updated_at;

-- ============================================================
-- one_roster_connections (1 col)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE one_roster_connections DROP COLUMN IF EXISTS deleted_at;

-- ============================================================
-- one_roster_sync_logs (8 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.OneRosterSyncLog
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS created_at;
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS created_count;
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS error_count;
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS finished_at;
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS updated_at;
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS updated_count;
ALTER TABLE one_roster_sync_logs DROP COLUMN IF EXISTS workflow_state;

-- ============================================================
-- content_migrations (1 col)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE content_migrations DROP COLUMN IF EXISTS deleted_at;

-- ============================================================
-- audit_logs (3 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE audit_logs DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.AuditLog
ALTER TABLE audit_logs DROP COLUMN IF EXISTS data;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS updated_at;

-- ============================================================
-- grade_change_logs (5 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE grade_change_logs DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.GradeChangeLog
ALTER TABLE grade_change_logs DROP COLUMN IF EXISTS excused_after;
ALTER TABLE grade_change_logs DROP COLUMN IF EXISTS excused_before;
ALTER TABLE grade_change_logs DROP COLUMN IF EXISTS graded_anonymously;
ALTER TABLE grade_change_logs DROP COLUMN IF EXISTS updated_at;

-- ============================================================
-- pii_access_logs (6 cols)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE pii_access_logs DROP COLUMN IF EXISTS deleted_at;
-- UNKNOWN — no matching field in models.PIIAccessLog
ALTER TABLE pii_access_logs DROP COLUMN IF EXISTS accessed_by;
ALTER TABLE pii_access_logs DROP COLUMN IF EXISTS data_accessed;
ALTER TABLE pii_access_logs DROP COLUMN IF EXISTS purpose;
ALTER TABLE pii_access_logs DROP COLUMN IF EXISTS updated_at;
ALTER TABLE pii_access_logs DROP COLUMN IF EXISTS user_id;

-- ============================================================
-- custom_roles (1 col)
-- ============================================================
-- SOFT_DELETE_LEFTOVER
ALTER TABLE custom_roles DROP COLUMN IF EXISTS deleted_at;

-- ============================================================
-- role_overrides (2 cols)
-- ============================================================
-- Wave 2b source (data copied to role_id in 000023)
ALTER TABLE role_overrides DROP COLUMN IF EXISTS custom_role_id;
-- SOFT_DELETE_LEFTOVER
ALTER TABLE role_overrides DROP COLUMN IF EXISTS deleted_at;

COMMIT;
