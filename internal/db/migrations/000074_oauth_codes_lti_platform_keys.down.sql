-- 000083_oauth_codes_lti_platform_keys.down.sql

BEGIN;

DROP TABLE IF EXISTS lti_platform_keys;
DROP TABLE IF EXISTS oauth_authorization_codes;

COMMIT;
