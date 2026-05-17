-- 000060_backfill_ldap_bind_password_encrypted.down.sql
--
-- The up migration only touches updated_at to mark backfill candidates;
-- there's nothing to revert at the SQL layer. The Go-side encryption
-- of plaintext rows is one-way by design: once the ciphertext column
-- is populated, the down direction would require decrypting and
-- re-storing plaintext, which is precisely the security posture this
-- migration is closing. Operators rolling back must restore plaintext
-- from a pre-rotation backup rather than running an automated down.

BEGIN;

-- intentional no-op

COMMIT;
