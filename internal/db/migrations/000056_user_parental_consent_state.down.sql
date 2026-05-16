-- 000056_user_parental_consent_state.down.sql

BEGIN;

ALTER TABLE users
    DROP COLUMN IF EXISTS requires_parental_consent;

COMMIT;
