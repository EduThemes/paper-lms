-- 000051_observer_pairing_codes.down.sql
BEGIN;

ALTER TABLE enrollments
    DROP CONSTRAINT IF EXISTS enrollments_associated_user_id_fkey;

COMMIT;
