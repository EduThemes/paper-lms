-- Reverse of 000040. Drops the per-learner leaderboard opt-out column.
-- ANY learner who had opted out loses that preference on rollback;
-- they default back to opted-IN. Operators should communicate this
-- before reverting in production.

BEGIN;

DROP INDEX IF EXISTS idx_users_leaderboard_opt_out;

ALTER TABLE users
    DROP COLUMN IF EXISTS leaderboard_opt_out;

COMMIT;
