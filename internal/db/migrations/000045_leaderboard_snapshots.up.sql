-- 000045_leaderboard_snapshots.up.sql
--
-- Phase 7 Sprint 7-B — weekly leaderboard snapshots.
--
-- The behavioral research (docs/research/gamification-2026-05/03-claude-
-- behavioral.md:277) treats "time-bounded resets, weekly, giving every
-- user a fresh chance" as a first-class motivational mechanic, not
-- polish. Wave 3 closed with live all-time ranking; this migration
-- adds the persistence shape that lets us serve "last week's
-- standings" without re-walking the wallet ledger on every read.
--
-- Shape decisions:
--
--   * `payload` is JSONB — the snapshot is read-only after window
--     close, never queried by inner field, and almost always
--     paginated in 5-50-row chunks. A child table would add an index
--     per (snapshot_id, rank) and double the row count for no read
--     benefit. JSONB keeps the payload localized to one row, one
--     cache hit.
--
--   * `scope_type` accepts the same `gamification_scope_type` enum as
--     gamification_rules.scope_type / gamification_currency_types.scope_type.
--     Pinning the type at the DB layer here means a future "snapshot
--     a section" feature reuses the existing enum cast rather than
--     drifting into the text-with-CHECK shape gamification_badges
--     was stuck in before migration 000044.
--
--   * `window_kind` is `text` with a CHECK constraint. v1 ships only
--     'weekly'; v2 will add 'monthly' / 'term'. Keeping it text +
--     CHECK lets us widen without an enum-cast migration.
--
--   * The UNIQUE on (scope, currency, kind, window_end) is the
--     idempotency surface. ComputeAndStore uses ON CONFLICT DO
--     NOTHING against this constraint so re-running the CLI for the
--     same window is a no-op — operationally safe to call from cron
--     with retries.
--
--   * No FK to gamification_currency_types(id) intentionally — see
--     F1.2 in the 2026-05-15 audit. Following the rest of the
--     gamification chain's pattern; FK backfill is its own follow-up
--     sprint. ON DELETE behavior of a currency is "RESTRICT" in the
--     wallet path (000034), so dangling snapshot pointers cannot
--     accumulate at runtime — only via raw SQL.

BEGIN;

CREATE TABLE IF NOT EXISTS gamification_leaderboard_snapshots (
    id                 bigserial PRIMARY KEY,
    scope_type         gamification_scope_type NOT NULL,
    scope_id           bigint                  NOT NULL,
    currency_type_id   bigint                  NOT NULL,
    window_kind        text                    NOT NULL
        CHECK (window_kind IN ('weekly')),
    window_start       timestamptz             NOT NULL,
    window_end         timestamptz             NOT NULL,
    computed_at        timestamptz             NOT NULL DEFAULT now(),
    payload            jsonb                   NOT NULL,
    -- Sanity: a window can't end before it began.
    CHECK (window_end > window_start)
);

-- Idempotency surface — ComputeAndStore writes ON CONFLICT DO NOTHING
-- against this constraint. One row per (scope, currency, kind, end).
CREATE UNIQUE INDEX IF NOT EXISTS idx_gam_lb_snapshot_window
    ON gamification_leaderboard_snapshots
       (scope_type, scope_id, currency_type_id, window_kind, window_end);

-- Read-path indexes:
--   * Most reads are "give me the most-recent window for this
--     (scope, currency, kind)" → covered by the UNIQUE above (the
--     planner can scan it DESC for the lookup).
--   * "List all snapshots for a tenant" comes up in operational
--     audit / cleanup; a separate index keeps it cheap.
CREATE INDEX IF NOT EXISTS idx_gam_lb_snapshot_scope
    ON gamification_leaderboard_snapshots
       (scope_type, scope_id, window_end DESC);

COMMIT;
