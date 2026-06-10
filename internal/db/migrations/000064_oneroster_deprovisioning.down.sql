-- 000064_oneroster_deprovisioning.down.sql

BEGIN;

ALTER TABLE one_roster_connections
    DROP COLUMN IF EXISTS deprovision_enabled;

ALTER TABLE one_roster_sync_logs
    DROP COLUMN IF EXISTS users_deprovisioned,
    DROP COLUMN IF EXISTS users_reactivated,
    DROP COLUMN IF EXISTS deprovision_aborted_reason;

ALTER TABLE users
    DROP COLUMN IF EXISTS suspended_by_sis;

COMMIT;
