-- Reverse of 000042. Leaderboard queries fall back to sequential scans
-- without these indexes; the data they index isn't dropped.

BEGIN;

DROP INDEX IF EXISTS idx_wallet_balances_currency_lifetime;
DROP INDEX IF EXISTS idx_enrollments_course_active;

COMMIT;
