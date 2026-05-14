-- 000040_users_leaderboard_opt_out.up.sql
--
-- Phase 6 Wave 2 Sprint W2-C — per-learner leaderboard opt-out.
--
-- Adds the privacy primitive that lets a learner remove themselves from
-- public leaderboard surfaces without losing XP / awards / mastery
-- progress. Per SYNTHESIS §5: opting out does NOT reduce the learner's
-- accumulated currencies — it just hides them from rankings displayed to
-- peers. Confirmed whitespace vs Brightspace's admin-only `MaskUsernames`
-- toggle, which lacks per-learner control.
--
-- No leaderboard surface ships in W2-C (the first leaderboard lands in
-- Wave 3). Shipping the column + filter helper now means Wave 3 starts
-- from a state where every leaderboard query path can call
-- UserRepository.FilterPublicLeaderboardCandidates against the existing
-- opt-out set rather than retrofitting the privacy guard later.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS leaderboard_opt_out boolean NOT NULL DEFAULT FALSE;

-- Partial index for the (small) opted-out subset. Leaderboard queries
-- expect the opt-out set to be a minority; a partial index gives O(N)
-- scans of just those rows for the filter helper's "EXCEPT" semantics.
CREATE INDEX IF NOT EXISTS idx_users_leaderboard_opt_out
    ON users (id)
    WHERE leaderboard_opt_out;

COMMIT;
