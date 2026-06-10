-- 000064_oneroster_deprovisioning.up.sql
--
-- Roster-driven auto-deprovisioning (K-12 non-negotiable): when a
-- OneRoster full sync finds a managed user missing from the source
-- roster, the sync suspends them (and unsuspends users who reappear).
--
-- `deprovision_enabled` is the per-connection opt-in — DEFAULT FALSE so
-- every existing connection keeps today's additive-only behavior until
-- a district admin explicitly turns it on.
--
-- The sync-log columns make every deprovision pass auditable: how many
-- users were suspended/reactivated, and — when the circuit breaker
-- refuses an implausibly large suspension delta — why nothing happened.

BEGIN;

ALTER TABLE one_roster_connections
    ADD COLUMN IF NOT EXISTS deprovision_enabled boolean NOT NULL DEFAULT false;

ALTER TABLE one_roster_sync_logs
    ADD COLUMN IF NOT EXISTS users_deprovisioned integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS users_reactivated integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS deprovision_aborted_reason text;

-- Suspension provenance: TRUE only when the deprovision pass set
-- `suspended`. Reactivation (user reappears in the roster) is limited
-- to rows with this flag, so a manual/disciplinary admin suspension is
-- never silently undone by a sync. Manual suspend/unsuspend clears it.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS suspended_by_sis boolean NOT NULL DEFAULT false;

COMMIT;
