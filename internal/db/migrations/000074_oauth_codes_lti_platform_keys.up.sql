-- 000074_oauth_codes_lti_platform_keys.up.sql
--
-- Integrations parity wave.
--
-- oauth_authorization_codes (B192): OAuth2 authorization codes used to
-- live in a per-process sync.Map that was never purged, so a code minted
-- on one replica could not be redeemed on another and abandoned codes
-- leaked memory. Codes now live here (SHA-256 of the code only), are
-- consumed with DELETE ... RETURNING, and expired rows are purged on
-- every new authorization.
--
-- lti_platform_keys (B181): the LTI 1.3 platform signing key used to be
-- generated in memory at boot, so every restart (and every replica)
-- had a different key and kid. The key now persists here, sealed with
-- secretbox (MFA_ENCRYPTION_KEY); `name` is a singleton key so racing
-- replicas converge on one row.

BEGIN;

CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    id bigserial,
    code_hash text NOT NULL,
    user_id bigint NOT NULL,
    developer_key_id bigint NOT NULL,
    client_id text NOT NULL,
    redirect_uri text NOT NULL,
    scopes text,
    expires_at timestamptz NOT NULL,
    created_at timestamptz,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_authorization_codes_code_hash ON public.oauth_authorization_codes USING btree (code_hash);
CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_developer_key_id ON public.oauth_authorization_codes USING btree (developer_key_id);
CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_expires_at ON public.oauth_authorization_codes USING btree (expires_at);
CREATE INDEX IF NOT EXISTS idx_oauth_authorization_codes_user_id ON public.oauth_authorization_codes USING btree (user_id);

CREATE TABLE IF NOT EXISTS lti_platform_keys (
    id bigserial,
    name text NOT NULL,
    key_id text NOT NULL,
    private_key_encrypted bytea NOT NULL,
    created_at timestamptz,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_lti_platform_keys_name ON public.lti_platform_keys USING btree (name);

COMMIT;
