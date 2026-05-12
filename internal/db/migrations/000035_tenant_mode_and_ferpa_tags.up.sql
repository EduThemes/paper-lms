-- 000035_tenant_mode_and_ferpa_tags.up.sql
--
-- Phase 6 Wave 1, compliance plumbing. Two pieces:
--
-- 1. accounts.tenant_mode + accounts.coppa_strict — the first thing the
--    gamification engine reads. K-12 vs HigherEd vs Corporate flips every
--    default toggle (friend streaks, public leaderboards, badge export,
--    behavioral profiling). Existing rows land as 'higher_ed'; K-12
--    tenants migrate manually or via a Wave 2 one-shot.
--
-- 2. gamification_ferpa_field_tags — lookup table mapping
--    (object_type, field_path_in_event_jsonb) → FERPA classification. The
--    FERPA guard (Wave 1 task 11, later PR) consults this on every Emit
--    to enforce that education_record-classified data never leaks into a
--    non_PII flagged context.

BEGIN;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS tenant_mode  gamification_audience NOT NULL DEFAULT 'higher_ed',
    ADD COLUMN IF NOT EXISTS coppa_strict boolean               NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS gamification_ferpa_field_tags (
    object_type    text NOT NULL,
    field_path     text NOT NULL,
    classification text NOT NULL CHECK (classification IN
        ('directory_information','education_record','non_PII','instructor_metadata')),
    description    text,
    created_at     timestamptz NOT NULL DEFAULT NOW(),
    updated_at     timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (object_type, field_path)
);

COMMIT;
