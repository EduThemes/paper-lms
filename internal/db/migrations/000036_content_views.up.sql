-- 000036_content_views.up.sql
--
-- Phase 6 Wave 1 / Sprint C: per-user content view aggregates.
--
-- `content_views` is a read-path optimization on top of the
-- `gamification_events` raw stream. Every page render emits a
-- `verb=viewed` event *and* upserts the matching row here, so the
-- `ViewedContent` predicate can answer "has this user viewed page P at
-- least N times" in one indexed read instead of counting events.
--
-- Not gamification-prefixed: this is a general LMS primitive (like
-- `submissions`) that gamification consumes. Other features (reading
-- comprehension dashboards, "continue where you left off") will read the
-- same table.
--
-- ID strategy: bigserial PK with a UNIQUE (user_id, object_type, object_id)
-- constraint so the IncrementView upsert has a deterministic target. The
-- composite index covers the read pattern the snapshot loader uses
-- (filter by user_id and a small set of object_ids).
--
-- See docs/research/gamification-2026-05/PHASE6-WAVE1-PLAN.md and the
-- Sprint C plan for the predicate-side consumer.

BEGIN;

CREATE TABLE IF NOT EXISTS content_views (
    id                bigserial   PRIMARY KEY,
    user_id           bigint      NOT NULL,
    object_type       text        NOT NULL,
    object_id         bigint      NOT NULL,
    view_count        integer     NOT NULL DEFAULT 1,
    total_seconds     bigint      NOT NULL DEFAULT 0,
    first_viewed_at   timestamptz NOT NULL DEFAULT NOW(),
    last_viewed_at    timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT uniq_content_views_user_object UNIQUE (user_id, object_type, object_id)
);

CREATE INDEX IF NOT EXISTS idx_content_views_user_object
    ON content_views (user_id, object_type, object_id);

CREATE INDEX IF NOT EXISTS idx_content_views_user_last
    ON content_views (user_id, last_viewed_at DESC);

COMMIT;
