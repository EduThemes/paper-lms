-- Reverse of 000045. Snapshots are append-only operational history;
-- dropping the table loses every stored window. Operators should
-- export to backup before reverting in production.

BEGIN;

DROP INDEX IF EXISTS idx_gam_lb_snapshot_scope;
DROP INDEX IF EXISTS idx_gam_lb_snapshot_window;
DROP TABLE IF EXISTS gamification_leaderboard_snapshots;

COMMIT;
