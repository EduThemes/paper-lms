-- 000075_hash_developer_key_secrets.up.sql
--
-- B191: OAuth2 client secrets were stored in plaintext in
-- developer_keys.client_secret. The secret is only ever compared (it is
-- shown to the admin once, at creation), so it is now stored as
-- 'sha256:' || hex(sha256(secret)) and verified in constant time
-- (service.VerifyClientSecret). This rewrites every legacy plaintext row
-- into that form; existing clients keep working because verification
-- hashes the presented secret the same way. Idempotent: rows already
-- carrying the prefix are left alone.

BEGIN;

UPDATE developer_keys
SET client_secret = 'sha256:' || encode(sha256(convert_to(client_secret, 'UTF8')), 'hex')
WHERE client_secret IS NOT NULL
  AND client_secret <> ''
  AND client_secret NOT LIKE 'sha256:%';

COMMIT;
