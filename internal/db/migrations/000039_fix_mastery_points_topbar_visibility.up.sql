-- Phase 6 Wave 2 / W2-A correctness backfill.
--
-- Repairs system-owned currency rows seeded via the buggy
-- `SeedSystemCurrenciesForTenant` code path that ran before W2-A landed
-- the raw-SQL rewrite. Two fields were affected by GORM's `default:`-tag
-- behavior, which elides zero-valued bool inserts in favor of the column
-- DEFAULT — flipping fields the seed declares as `false` to `true`:
--
--   * `mastery_points.visible_in_topbar` — should be FALSE per
--     SYNTHESIS §2's FERPA contract; was silently TRUE.
--   * `gems.monotonic`                   — should be FALSE per the
--     four-currency design (gems are spendable, balance can decrease);
--     was silently TRUE.
--
-- The Go-side fix is a raw INSERT in the seed that writes every column
-- explicitly. This migration cleans up rows already written through the
-- buggy path. Idempotent; a tenant seeded post-fix has these flags
-- correct already and the WHEREs are no-ops there.

BEGIN;

UPDATE gamification_currency_types
SET visible_in_topbar = FALSE
WHERE code            = 'mastery_points'
  AND system_owned    = TRUE
  AND visible_in_topbar = TRUE;

UPDATE gamification_currency_types
SET monotonic = FALSE
WHERE code            = 'gems'
  AND system_owned    = TRUE
  AND monotonic = TRUE;

COMMIT;
