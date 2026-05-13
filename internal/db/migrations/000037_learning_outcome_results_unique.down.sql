-- 000037_learning_outcome_results_unique.down.sql
--
-- Drop the UNIQUE constraint added by the up migration. The dedup
-- DELETEs are NOT reversed — there's no recovery path for the rows
-- that were intentionally removed, and the constraint dropping doesn't
-- re-introduce dupes.

BEGIN;

ALTER TABLE learning_outcome_results
  DROP CONSTRAINT IF EXISTS uniq_lor_user_outcome_asset;

COMMIT;
