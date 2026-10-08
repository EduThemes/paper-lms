-- 000096_hash_developer_key_secrets.down.sql
--
-- Irreversible by design: a SHA-256 digest cannot be turned back into
-- the client secret. Rolling back the code is still safe — affected
-- developer keys simply need a new secret (recreate the key).

SELECT 1;
