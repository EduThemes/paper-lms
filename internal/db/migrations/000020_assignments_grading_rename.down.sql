-- Reverse the data copies from the .up.sql. Each statement back-populates
-- the legacy column from the new one when the legacy column is at its zero
-- value (false for booleans), mirroring the up direction.
--
-- This is a bool→bool rename in both cases; no information is lost on
-- rollback. The legacy columns remain in place until Wave 2c drops them.

BEGIN;

-- Reverse 2: display_totals → display_totals_for_all_grading_periods.
UPDATE grading_period_groups
SET display_totals_for_all_grading_periods = true
WHERE display_totals_for_all_grading_periods = false
  AND display_totals = true;

-- Reverse 1: peer_reviews_enabled → peer_reviews.
UPDATE assignments
SET peer_reviews = true
WHERE peer_reviews = false
  AND peer_reviews_enabled = true;

COMMIT;
