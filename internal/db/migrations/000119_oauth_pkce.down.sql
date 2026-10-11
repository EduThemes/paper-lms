BEGIN;

ALTER TABLE oauth_authorization_codes DROP COLUMN IF EXISTS code_challenge_method;
ALTER TABLE oauth_authorization_codes DROP COLUMN IF EXISTS code_challenge;

COMMIT;
