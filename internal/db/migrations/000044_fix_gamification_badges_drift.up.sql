-- 000044_fix_gamification_badges_drift.up.sql
--
-- Phase 7 Sprint 7-A audit remediation:
--
--   * F1.4 — `gamification_badges.scope_type` was `text NOT NULL` while
--     the analogous columns on `gamification_currency_types` and
--     `gamification_rules` use the `gamification_scope_type` enum. A
--     typo in code or a stray INSERT could land 'coarse' (or empty
--     string) in the DB. CHECK constraint here as a first hardening
--     step; full enum cast is a follow-up migration once we've
--     verified no existing rows are out-of-range.
--
--   * F1.9 — five text columns shipped as `NOT NULL DEFAULT ''`, which
--     conflates "no value set" with "empty value set." Two real costs:
--     - The application can't tell whether an admin meant "(blank)" or
--       "(not yet authored)".
--     - `audience_level` is reserved for future audience-filter rules;
--       if those land on a partial UNIQUE index, the empty-string-vs-NULL
--       class of bug fires exactly like W3-B's pseudonym_name did
--       before its *string fix.
--     Relax to nullable. Existing '' values stay; new code can write NULL.
--
--   * F1.11 — `audience_level` is the same logical column as
--     `gamification_rules.audience_level` (which uses the
--     `gamification_audience` enum). Aligning to the same enum is
--     deferred to the W7-B+ follow-up; today we just nullify so future
--     callers aren't forced into the '' sentinel.

BEGIN;

-- F1.4 — scope_type hardening (CHECK first, enum cast later).
ALTER TABLE gamification_badges
    ADD CONSTRAINT chk_gam_badges_scope_type
    CHECK (scope_type IN ('site', 'district', 'school', 'course', 'section'));

-- F1.9 + F1.11 — relax the five oversized NOT NULL DEFAULT '' columns.
ALTER TABLE gamification_badges
    ALTER COLUMN description    DROP NOT NULL,
    ALTER COLUMN description    DROP DEFAULT;
ALTER TABLE gamification_badges
    ALTER COLUMN icon           DROP NOT NULL,
    ALTER COLUMN icon           DROP DEFAULT;
ALTER TABLE gamification_badges
    ALTER COLUMN image_url      DROP NOT NULL,
    ALTER COLUMN image_url      DROP DEFAULT;
ALTER TABLE gamification_badges
    ALTER COLUMN color          DROP NOT NULL,
    ALTER COLUMN color          DROP DEFAULT;
ALTER TABLE gamification_badges
    ALTER COLUMN audience_level DROP NOT NULL,
    ALTER COLUMN audience_level DROP DEFAULT;

COMMIT;
