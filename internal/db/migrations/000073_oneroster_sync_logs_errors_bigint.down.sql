-- 000093_oneroster_sync_logs_errors_bigint.down.sql

BEGIN;

ALTER TABLE one_roster_sync_logs
    ALTER COLUMN errors DROP DEFAULT;

ALTER TABLE one_roster_sync_logs
    ALTER COLUMN errors TYPE text USING errors::text;

COMMIT;
