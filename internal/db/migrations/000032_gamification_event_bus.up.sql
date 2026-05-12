-- 000032_gamification_event_bus.up.sql
--
-- Phase 6 Wave 1, table 1 of 5: the xAPI-shaped event store. Every
-- gamification-relevant action in Paper LMS emits one row here. The rules
-- engine (000033) subscribes to this stream; effects (currency awards,
-- badges, content release) are consumers.
--
-- ID strategy: gamification_events.id is a BIGSERIAL to match the rest of
-- the Paper LMS schema (users.id, accounts.id, courses.id are all bigint).
-- The xAPI export layer can synthesize an IRI/UUID at serialization time;
-- the internal substrate stays integer-keyed so joins to existing tables
-- are natural and parity-test-clean.
--
-- See docs/research/gamification-2026-05/PHASE6-WAVE1-PLAN.md §"Migration
-- plan" for the source spec, and §"Option A" decision (2026-05-12) for why
-- the plan's UUID example was relaxed to bigint here.

BEGIN;

CREATE TABLE IF NOT EXISTS gamification_events (
    id                bigserial PRIMARY KEY,
    occurred_at       timestamptz NOT NULL,
    emitted_at        timestamptz NOT NULL DEFAULT NOW(),
    tenant_id         bigint      NOT NULL,
    actor_id          bigint      NOT NULL,
    verb              text        NOT NULL,
    object_type       text        NOT NULL,
    object_id         bigint,
    result            jsonb,
    context           jsonb,
    source            text        NOT NULL DEFAULT 'internal',
    source_event_id   text,
    policy_flags      text[]      NOT NULL DEFAULT '{}',
    signature         text
);

CREATE INDEX IF NOT EXISTS idx_gam_events_actor_occurred
    ON gamification_events (actor_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_gam_events_verb_object
    ON gamification_events (verb, object_type, object_id);

CREATE INDEX IF NOT EXISTS idx_gam_events_tenant_time
    ON gamification_events (tenant_id, occurred_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_gam_events_source_event_id
    ON gamification_events (source, source_event_id)
    WHERE source_event_id IS NOT NULL;

COMMIT;
