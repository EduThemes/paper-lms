-- Reverse of 000046. Destructive: drops federation+MFA+recovery state.
-- Operators with live MFA enrollments must export before reverting.

BEGIN;

ALTER TABLE authentication_providers
    DROP COLUMN IF EXISTS auto_provision,
    DROP COLUMN IF EXISTS ldap_bind_password_encrypted;

DROP INDEX IF EXISTS idx_user_recovery_codes_user_unused;
DROP TABLE IF EXISTS user_recovery_codes;

ALTER TABLE users
    DROP COLUMN IF EXISTS totp_verified_at,
    DROP COLUMN IF EXISTS totp_secret_encrypted,
    DROP COLUMN IF EXISTS webauthn_user_handle;

ALTER TABLE accounts
    DROP CONSTRAINT IF EXISTS accounts_mfa_policy_check,
    DROP COLUMN IF EXISTS mfa_policy;

DROP INDEX IF EXISTS idx_federated_identities_user;
DROP TABLE IF EXISTS federated_identities;

-- pgcrypto stays installed; other migrations may depend on it.

COMMIT;
