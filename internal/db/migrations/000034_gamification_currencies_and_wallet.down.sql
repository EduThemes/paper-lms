-- Reverse of 000034. Drops wallet transactions, balances, then currency
-- types. Row data is LOST. Currency seeds (xp/gems/mastery_points/
-- reputation) will be re-created on next tenant.creation hook or seeder
-- run after a forward re-apply.

BEGIN;

DROP INDEX IF EXISTS idx_gam_wallet_tx_event;
DROP INDEX IF EXISTS idx_gam_wallet_tx_rule;
DROP INDEX IF EXISTS idx_gam_wallet_tx_currency_time;
DROP INDEX IF EXISTS idx_gam_wallet_tx_user_time;

DROP TABLE IF EXISTS gamification_wallet_transactions;

DROP INDEX IF EXISTS idx_gam_wallet_balances_currency;

DROP TABLE IF EXISTS gamification_wallet_balances;

DROP INDEX IF EXISTS idx_gam_currency_topbar;
DROP INDEX IF EXISTS idx_gam_currency_tenant_scope;

DROP TABLE IF EXISTS gamification_currency_types;

COMMIT;
