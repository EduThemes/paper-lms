-- 000055_account_default_locale.up.sql
--
-- Phase 13 Item 13.11 — i18n surface.
--
-- The audit found react-i18next is configured but unused; Spanish-speaking
-- Kansas districts cannot adopt without per-tenant locale plumbing. The
-- minimum-viable piece is a tenant-level default locale that the frontend
-- can read at session bootstrap. Per-page string extraction + translator
-- pass is the engineering follow-up.

BEGIN;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS default_locale text NOT NULL DEFAULT 'en';

COMMIT;
