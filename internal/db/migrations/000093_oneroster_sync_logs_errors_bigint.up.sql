-- 000093_oneroster_sync_logs_errors_bigint.up.sql
--
-- one_roster_sync_logs.errors was created as `text` by 000001 (a legacy
-- free-form error column), but the OneRosterSyncLog model has always
-- mapped it to an integer error count. On a database built from the SQL
-- chain every sync-log INSERT failed ("unable to encode 0 into text"),
-- so no OneRoster sync could start at all — manual or the auto-sync job
-- added for B173. The free-form details live in error_details.
--
-- Convert the column to the bigint count the model writes. Any legacy
-- non-numeric text becomes 0 (the details were never readable through
-- the API, which serializes the int field).

BEGIN;

ALTER TABLE one_roster_sync_logs
    ALTER COLUMN errors DROP DEFAULT;

ALTER TABLE one_roster_sync_logs
    ALTER COLUMN errors TYPE bigint
    USING (CASE WHEN errors ~ '^[0-9]+$' THEN errors::bigint ELSE 0 END);

ALTER TABLE one_roster_sync_logs
    ALTER COLUMN errors SET DEFAULT 0;

COMMIT;
