-- Reverse of 000033. Drops rules + evaluations, then the two enums. The
-- enums are referenced by gamification_currency_types (000034) and
-- accounts.tenant_mode (000035), so 000034 and 000035 must be rolled back
-- before this migration. Row data is LOST.

BEGIN;

DROP INDEX IF EXISTS uniq_gam_eval_rule_user_time;
DROP INDEX IF EXISTS idx_gam_eval_rule_time;
DROP INDEX IF EXISTS idx_gam_eval_user_rule_time;

DROP TABLE IF EXISTS gamification_rule_evaluations;

DROP INDEX IF EXISTS idx_gam_rules_tenant;
DROP INDEX IF EXISTS idx_gam_rules_scope;

DROP TABLE IF EXISTS gamification_rules;

DROP TYPE IF EXISTS gamification_audience;
DROP TYPE IF EXISTS gamification_scope_type;

COMMIT;
