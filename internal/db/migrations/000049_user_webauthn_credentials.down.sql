-- Reverse of 000049. Drops the passkey credential table.

BEGIN;

DROP TABLE IF EXISTS user_webauthn_credentials;

COMMIT;
