-- 000037_learning_outcome_results_unique.up.sql
--
-- Phase 6 Wave 1 Sprint D-3 — close the residual INSERT-side mastery
-- race documented on PR #10. PR #10's row-locked SELECT … FOR UPDATE
-- closes the update-side; the insert-side needs DB-enforced uniqueness
-- so two concurrent first-time writes can't both succeed and both fire
-- OnMasteryCrossed.
--
-- The repository layer pairs this constraint with ON CONFLICT DO NOTHING
-- on the Create path: the loser of the race re-fetches under the row
-- lock and falls through to the update path, correctly observing the
-- newly-inserted row as the "prior" state for transition detection.

BEGIN;

-- Defensive deduplication: the schema didn't enforce uniqueness before,
-- so prior dupes are theoretically possible from concurrent CreateResult
-- calls. Keep the most-recently-created row per composite; tie-breaker
-- on lower id when created_at matches.
DELETE FROM learning_outcome_results a
USING learning_outcome_results b
WHERE a.user_id = b.user_id
  AND a.learning_outcome_id = b.learning_outcome_id
  AND a.associated_asset_type = b.associated_asset_type
  AND a.associated_asset_id = b.associated_asset_id
  AND a.created_at < b.created_at;

DELETE FROM learning_outcome_results a
USING learning_outcome_results b
WHERE a.user_id = b.user_id
  AND a.learning_outcome_id = b.learning_outcome_id
  AND a.associated_asset_type = b.associated_asset_type
  AND a.associated_asset_id = b.associated_asset_id
  AND a.created_at = b.created_at
  AND a.id > b.id;

ALTER TABLE learning_outcome_results
  ADD CONSTRAINT uniq_lor_user_outcome_asset
  UNIQUE (user_id, learning_outcome_id, associated_asset_type, associated_asset_id);

COMMIT;
