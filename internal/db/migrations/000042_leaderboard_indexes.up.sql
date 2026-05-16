-- 000042_leaderboard_indexes.up.sql
--
-- Phase 6 Wave 3 Sprint W3-A — leaderboard ranking indexes.
--
-- No new tables yet; W3-A ranks live from gamification_wallet_balances
-- using lifetime_earned (monotonic, survives spends — see
-- gamification_wallet.go:9–14 for the rationale). These two indexes
-- keep the per-currency top-N + per-course candidate query cheap:
--
--   * idx_wallet_balances_currency_lifetime supports
--     `WHERE currency_type_id = ? ORDER BY lifetime_earned DESC` —
--     the spine of every leaderboard query.
--   * idx_enrollments_course_active is a partial index over the
--     active-enrollment subset, which is what the course-scoped
--     leaderboard joins against to bound the candidate set. The
--     workflow_state = 'active' predicate matches the FilterPublic-
--     LeaderboardCandidates call path (W2-C) which expects the
--     caller to pre-narrow to active members.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_wallet_balances_currency_lifetime
    ON gamification_wallet_balances (currency_type_id, lifetime_earned DESC);

CREATE INDEX IF NOT EXISTS idx_enrollments_course_active
    ON enrollments (course_id)
    WHERE workflow_state = 'active';

COMMIT;
