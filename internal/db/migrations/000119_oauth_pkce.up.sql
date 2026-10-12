-- 000119_oauth_pkce.up.sql
--
-- G136: RFC 7636 (PKCE) for the OAuth2 provider. The challenge a client
-- sends to /login/oauth2/auth is stored with the authorization code and
-- checked against the code_verifier at /login/oauth2/token. Both columns
-- are NULL for a plain (non-PKCE) authorization-code request.

BEGIN;

ALTER TABLE oauth_authorization_codes ADD COLUMN IF NOT EXISTS code_challenge text;
ALTER TABLE oauth_authorization_codes ADD COLUMN IF NOT EXISTS code_challenge_method text;

COMMIT;
