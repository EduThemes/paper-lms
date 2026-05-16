-- 000056_user_parental_consent_state.up.sql
--
-- Phase 13 Item 13.4 (Wave C.2) — COPPA signup gate.
--
-- When a user signs up to a coppa_strict tenant and their age verification
-- indicates is_under_13, the user row is created but the workflow_state is
-- "pending_parental_consent" and this flag is set. The user cannot log in
-- (or be auto-logged-in post-Register) until a parent verifies the consent
-- token via the COPPA consent flow.
--
-- The flag is intentionally separate from workflow_state so a future state
-- machine (suspended, locked, etc.) can coexist without overloading a single
-- column.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS requires_parental_consent boolean NOT NULL DEFAULT false;

COMMIT;
