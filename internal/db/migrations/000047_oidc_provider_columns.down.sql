-- Reverse of 000047. Drops OIDC columns and the auth_type CHECK.
-- Any oidc-typed providers must be deleted before reverting or the
-- (now-uncovered) CHECK predicate will be re-applied if 047 is
-- re-applied later.

BEGIN;

ALTER TABLE authentication_providers
    DROP CONSTRAINT IF EXISTS authentication_providers_auth_type_check;

ALTER TABLE authentication_providers
    DROP COLUMN IF EXISTS oidc_preset,
    DROP COLUMN IF EXISTS oidc_scopes,
    DROP COLUMN IF EXISTS oidc_client_secret_encrypted,
    DROP COLUMN IF EXISTS oidc_client_id,
    DROP COLUMN IF EXISTS oidc_issuer_url;

COMMIT;
