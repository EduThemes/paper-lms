-- 000047_oidc_provider_columns.up.sql
--
-- Phase 9 Sprint 9-A — OIDC client mode.
--
-- Extends authentication_providers with the columns OIDC needs:
--   * oidc_issuer_url — the IdP's discovery base, e.g.
--     "https://accounts.google.com". The `coreos/go-oidc` library
--     reads /.well-known/openid-configuration from this URL.
--   * oidc_client_id — registered RP client identifier.
--   * oidc_client_secret_encrypted — AES-256-GCM ciphertext of the
--     client secret. Never stored plaintext (see CLAUDE.md "Phase 7
--     patterns" — same principle as TOTP secrets).
--   * oidc_scopes — array of scopes to request. Default
--     `{"openid","email","profile"}` covers every preset.
--   * oidc_preset — informational tag for which preset template the
--     admin started from (`google`, `microsoft`, `apple`, `generic`).
--     Drives small render-time hints in the admin UI; not load-bearing.
--
-- Also widens the auth_type CHECK to include 'oidc'. The existing
-- check accepts ('saml','ldap','cas'); add 'oidc' as the fourth.

BEGIN;

ALTER TABLE authentication_providers
    ADD COLUMN IF NOT EXISTS oidc_issuer_url               text,
    ADD COLUMN IF NOT EXISTS oidc_client_id                text,
    ADD COLUMN IF NOT EXISTS oidc_client_secret_encrypted  bytea,
    ADD COLUMN IF NOT EXISTS oidc_scopes                   text[] DEFAULT ARRAY['openid','email','profile'],
    ADD COLUMN IF NOT EXISTS oidc_preset                   text;

-- The auth_type column doesn't currently have a CHECK constraint in
-- the SQL chain (the Go model comment lists valid values but no DB
-- enforcement). Add one now to lock down the four supported types.
-- Idempotent via DO block.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'authentication_providers_auth_type_check'
    ) THEN
        ALTER TABLE authentication_providers
            ADD CONSTRAINT authentication_providers_auth_type_check
            CHECK (auth_type IN ('saml','ldap','cas','oidc'));
    END IF;
END$$;

COMMIT;
