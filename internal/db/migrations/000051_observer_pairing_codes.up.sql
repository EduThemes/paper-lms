-- 000051_observer_pairing_codes.up.sql
--
-- Phase 12 — Tier 1 university hardening, item 12.6.
--
-- Closes the LinkObservee IDOR (`POST /users/:user_id/observees`). The
-- handler-level remediation is to require a pairing-code handshake — that
-- flows through the EXISTING `pairing_codes` table from migration 000009
-- and `PairingCodeService.Redeem`. The schema change in this migration is
-- the missing piece: the `enrollments.associated_user_id` column was
-- declared by GORM long ago, but no FK was ever wired, so a deleted
-- student left a dangling parent->ghost-child observer link. The audit
-- flagged this as part of the "no FK enforcement on the Canvas-inherited
-- core" finding.

BEGIN;

-- Null out any stale associated_user_id values that point at users no
-- longer in the table — otherwise the FK constraint can't be created.
UPDATE enrollments
   SET associated_user_id = NULL
 WHERE associated_user_id IS NOT NULL
   AND NOT EXISTS (
       SELECT 1 FROM users u WHERE u.id = enrollments.associated_user_id
   );

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'enrollments_associated_user_id_fkey'
    ) THEN
        ALTER TABLE enrollments
            ADD CONSTRAINT enrollments_associated_user_id_fkey
            FOREIGN KEY (associated_user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END$$;

COMMIT;
