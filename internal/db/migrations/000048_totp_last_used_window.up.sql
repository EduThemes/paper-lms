-- 000048_totp_last_used_window.up.sql
--
-- Phase 10 Sprint 10-A.5 — TOTP code-reuse protection (RFC 6238 §5.2).
--
-- A TOTP code is valid for its 30-second window plus ±1 step (90s
-- total). Without tracking last-used-window per user, a code phished
-- via a fake login page can be replayed by both the attacker AND the
-- legitimate user inside that 90-second envelope.
--
-- Fix: store the most recently consumed TOTP step counter
-- (Unix-seconds / 30) on the user. On verify:
--   1. compute current_window = now.Unix() / 30
--   2. reject if user.totp_last_used_window >= current_window
--   3. on success, set user.totp_last_used_window = current_window
--
-- Stored as bigint so it survives past Y2038. Default 0 = "never
-- used" — every real TOTP code lands in a window > 0.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS totp_last_used_window bigint NOT NULL DEFAULT 0;

COMMIT;
