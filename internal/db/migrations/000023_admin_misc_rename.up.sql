-- Wave 2b: data migration for the admin + compliance + portfolios + misc domain.
--
-- Tables covered: student_accommodations, parental_consents,
-- data_processing_agreements, age_verifications, data_retention_policies,
-- data_deletion_requests, data_export_requests, portfolios, portfolio_sections,
-- portfolio_artifacts, portfolio_reflections, portfolio_templates,
-- portfolio_comments, group_categories, groups, group_memberships,
-- blueprint_templates, blueprint_subscriptions, blueprint_migrations,
-- sis_batches, sis_batch_errors, one_roster_connections, one_roster_sync_logs,
-- content_migrations, audit_logs, grade_change_logs, pii_access_logs,
-- custom_roles, role_overrides, feature_flags, content_embeddings,
-- shared_content, shared_content_favorites.
--
-- Action taken per table:
--   age_verifications        — COPY  is_under_13 → is_under13 (boolean rename)
--   portfolio_artifacts      — COPY  portfolio_section_id → section_id
--                                    attachment_id + submission_id → source_id/source_type
--   role_overrides           — COPY  custom_role_id → role_id (hidden rename,
--                                    reclassified from POLYMORPHIC_REFACTOR)
--
-- All other tables in this domain are DEFERRED:
--   student_accommodations   — all stale columns are SOFT_DELETE_LEFTOVER or
--                              UNKNOWN with no matching model field
--   parental_consents        — all stale columns are SOFT_DELETE_LEFTOVER or
--                              UNKNOWN with no matching model field
--   data_processing_agreements — same
--   data_retention_policies  — same
--   data_deletion_requests   — same
--   data_export_requests     — same
--   portfolios               — same
--   portfolio_sections       — same
--   portfolio_reflections    — same
--   portfolio_templates      — same
--   portfolio_comments       — same
--   group_categories         — same (context_id/context_type are UNKNOWN;
--                              the new model uses course_id/account_id directly)
--   groups                   — deleted_at only (SOFT_DELETE_LEFTOVER)
--   group_memberships        — deleted_at only (SOFT_DELETE_LEFTOVER)
--   blueprint_templates      — all SOFT_DELETE_LEFTOVER or UNKNOWN
--   blueprint_subscriptions  — deleted_at only (SOFT_DELETE_LEFTOVER)
--   blueprint_migrations     — all SOFT_DELETE_LEFTOVER or UNKNOWN
--   sis_batches              — all SOFT_DELETE_LEFTOVER or UNKNOWN
--   sis_batch_errors         — same
--   one_roster_connections   — deleted_at only (SOFT_DELETE_LEFTOVER)
--   one_roster_sync_logs     — all SOFT_DELETE_LEFTOVER or UNKNOWN
--   content_migrations       — deleted_at only (SOFT_DELETE_LEFTOVER)
--   audit_logs               — all SOFT_DELETE_LEFTOVER or UNKNOWN
--   grade_change_logs        — same
--   pii_access_logs          — same
--   custom_roles             — deleted_at only (SOFT_DELETE_LEFTOVER)
--   feature_flags            — no stale columns
--   content_embeddings       — no stale columns
--   shared_content           — no stale columns (visibility col is current model)
--   shared_content_favorites — no stale columns
--
-- All guards use GORM zero-values:
--   boolean NOT NULL columns: false = zero → copy only when source true and target false
--   bigint NOT NULL columns:  0    = zero → copy only when target = 0 and source > 0
--   nullable columns:         NULL = zero → copy only when target IS NULL and source IS NOT NULL
--
-- Every statement is idempotent.

BEGIN;

-- ============================================================
-- 1. age_verifications: is_under_13 (boolean nullable, default false)
--    → is_under13 (boolean NOT NULL, default false)
--
-- RENAME_CANDIDATE: the GORM model field IsUnder13 maps to is_under13
-- (Go naming convention strips the underscore before the digit).
-- Copy when the legacy column is true and the new column is still false.
-- ============================================================
UPDATE age_verifications
SET is_under13 = true
WHERE is_under_13 = true
  AND is_under13 = false;

-- ============================================================
-- 2. portfolio_artifacts: portfolio_section_id → section_id
--
-- Reclassified from POLYMORPHIC_REFACTOR (target listed as source_id) to
-- HIDDEN_RENAME: portfolio_section_id was the parent-section FK in the old
-- schema; the new model promotes this to section_id (direct bigint FK).
-- Copy when section_id is NULL and portfolio_section_id is set.
-- ============================================================
UPDATE portfolio_artifacts
SET section_id = portfolio_section_id
WHERE section_id IS NULL
  AND portfolio_section_id IS NOT NULL
  AND portfolio_section_id > 0;

-- ============================================================
-- 3. portfolio_artifacts: attachment_id → (source_id, source_type='upload')
--
-- POLYMORPHIC_REFACTOR. The old schema had attachment_id as a direct FK
-- to attachments; the new model uses a polymorphic (source_type, source_id)
-- pair. Copy when source_id is not yet set and attachment_id is present.
-- attachment_id and submission_id were mutually exclusive in the old schema,
-- so the priority order here is safe.
-- ============================================================
UPDATE portfolio_artifacts
SET source_id   = attachment_id,
    source_type = 'upload'
WHERE source_id IS NULL
  AND attachment_id IS NOT NULL
  AND attachment_id > 0;

-- ============================================================
-- 4. portfolio_artifacts: submission_id → (source_id, source_type='course_submission')
--
-- POLYMORPHIC_REFACTOR. Same polymorphic pair as above, for the submission
-- FK path. Only runs when source_id was not already populated by step 3.
-- ============================================================
UPDATE portfolio_artifacts
SET source_id   = submission_id,
    source_type = 'course_submission'
WHERE source_id IS NULL
  AND submission_id IS NOT NULL
  AND submission_id > 0;

-- ============================================================
-- 5. role_overrides: custom_role_id → role_id
--
-- Reclassified from POLYMORPHIC_REFACTOR (STALE_COLUMNS listed target as
-- context_id) to HIDDEN_RENAME: the original role_overrides table had only
-- custom_role_id as the FK to custom_roles. Wave 1 added role_id (NOT NULL)
-- and account_id + context_id for the new polymorphic context fields. The
-- semantic meaning of custom_role_id matches role_id directly.
-- role_id is bigint NOT NULL (default 0 from Wave 1 backfill).
-- ============================================================
UPDATE role_overrides
SET role_id = custom_role_id
WHERE role_id = 0
  AND custom_role_id IS NOT NULL
  AND custom_role_id > 0;

COMMIT;
