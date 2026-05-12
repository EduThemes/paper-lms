-- Reverse of 000032. Drops the event store. Row data is LOST.

BEGIN;

DROP INDEX IF EXISTS uniq_gam_events_source_event_id;
DROP INDEX IF EXISTS idx_gam_events_tenant_time;
DROP INDEX IF EXISTS idx_gam_events_verb_object;
DROP INDEX IF EXISTS idx_gam_events_actor_occurred;

DROP TABLE IF EXISTS gamification_events;

COMMIT;
