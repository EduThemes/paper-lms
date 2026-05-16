-- 000050_gamification_enums_and_fks.down.sql

BEGIN;

-- Drop FKs in reverse order of creation.
ALTER TABLE gamification_leaderboard_snapshots DROP CONSTRAINT IF EXISTS fk_gam_snapshots_tenant;
ALTER TABLE gamification_badge_awards          DROP CONSTRAINT IF EXISTS fk_gam_badge_awards_awarded_by;
ALTER TABLE gamification_badge_awards          DROP CONSTRAINT IF EXISTS fk_gam_badge_awards_user;
ALTER TABLE gamification_badges                DROP CONSTRAINT IF EXISTS fk_gam_badges_created_by;
ALTER TABLE gamification_badges                DROP CONSTRAINT IF EXISTS fk_gam_badges_tenant;
ALTER TABLE gamification_rule_evaluations      DROP CONSTRAINT IF EXISTS fk_gam_rule_evals_user;
ALTER TABLE gamification_rules                 DROP CONSTRAINT IF EXISTS fk_gam_rules_created_by;
ALTER TABLE gamification_rules                 DROP CONSTRAINT IF EXISTS fk_gam_rules_tenant;
ALTER TABLE gamification_wallet_transactions   DROP CONSTRAINT IF EXISTS fk_gam_wallet_tx_user;
ALTER TABLE gamification_wallet_balances       DROP CONSTRAINT IF EXISTS fk_gam_wallet_balances_user;
ALTER TABLE gamification_currency_types        DROP CONSTRAINT IF EXISTS fk_gam_currencies_tenant;
ALTER TABLE gamification_events                DROP CONSTRAINT IF EXISTS fk_gam_events_actor;
ALTER TABLE gamification_events                DROP CONSTRAINT IF EXISTS fk_gam_events_tenant;

-- Revert audience_level: enum → nullable text. Restore the empty-string
-- default would break the F1.11 fix — we leave it nullable.
ALTER TABLE gamification_badges
    ALTER COLUMN audience_level TYPE text
    USING audience_level::text;

-- Revert scope_type: enum → text + CHECK constraint (the pre-000050
-- shape post-000044).
ALTER TABLE gamification_badges
    ALTER COLUMN scope_type TYPE text
    USING scope_type::text;

ALTER TABLE gamification_badges
    ADD CONSTRAINT chk_gam_badges_scope_type
    CHECK (scope_type IN ('site', 'district', 'school', 'course', 'section'));

COMMIT;
