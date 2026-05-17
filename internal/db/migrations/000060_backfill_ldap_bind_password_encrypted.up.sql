-- 000060_backfill_ldap_bind_password_encrypted.up.sql
--
-- Phase 9-PRE encryption-at-rest contract enforcement (Wave A).
--
-- Migration 000046 added authentication_providers.ldap_bind_password_encrypted
-- and promised the plaintext ldap_bind_password column would be
-- "dropped after the Go-side backfill confirms it ran." The 9-PRE
-- backfill never wired up, so the plaintext column has been the de
-- facto storage path. This migration is the SQL-side half of closing
-- that gap; the encryption itself can't happen in SQL because the
-- AES-256-GCM key lives in the Go process (MFA_ENCRYPTION_KEY).
--
-- What this migration DOES NOT do:
--   * Drop ldap_bind_password. That's Wave-B, after every row has been
--     re-encrypted. Dropping here would lock plaintext rows out of
--     authentication on the next boot if the Go backfill hadn't yet
--     run, so we keep the column for one release as a safety net.
--   * Encrypt the existing rows. The AES-256-GCM key lives in the Go
--     process; we can't reach it from psql. The Go-side backfill at
--     boot does the actual seal.
--
-- What this migration DOES:
--   * Touch updated_at on rows that still have a plaintext bind
--     password and no encrypted column, so an operator looking at the
--     audit trail can see exactly which rows are awaiting backfill.
--   * Document the two-PR strategy in this header.
--
-- The companion Go-side backfill (auth.BackfillLDAPBindPasswords,
-- invoked from cmd/server/main.go right after auth.EnsureKeysLoaded)
-- iterates rows where ldap_bind_password != '' AND
-- ldap_bind_password_encrypted IS NULL, encrypts each plaintext via
-- secretbox.Encrypt, writes the ciphertext to the encrypted column,
-- and clears the plaintext column. Idempotent — no-ops on a clean DB.

BEGIN;

-- Idempotent marker bump. The WHERE clause makes this a no-op once
-- every row has been backfilled (encrypted column populated → row
-- excluded from the update set).
UPDATE authentication_providers
   SET updated_at = updated_at
 WHERE auth_type = 'ldap'
   AND ldap_bind_password IS NOT NULL
   AND ldap_bind_password <> ''
   AND (ldap_bind_password_encrypted IS NULL OR octet_length(ldap_bind_password_encrypted) = 0);

COMMIT;
