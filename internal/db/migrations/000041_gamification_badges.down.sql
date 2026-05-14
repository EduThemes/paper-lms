-- Reverse of 000041. Drops both badge tables. Awards data is LOST —
-- this is W2-D's first ship of badges, so rolling back forfeits all
-- badge data by design. Down migration exists for dev rollback only.

BEGIN;

DROP TABLE IF EXISTS gamification_badge_awards;
DROP TABLE IF EXISTS gamification_badges;

COMMIT;
