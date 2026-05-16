-- Reverse of 000048. Reverts to no-reuse-protection state.

BEGIN;

ALTER TABLE users DROP COLUMN IF EXISTS totp_last_used_window;

COMMIT;
