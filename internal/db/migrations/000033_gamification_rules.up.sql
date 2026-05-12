-- 000033_gamification_rules.up.sql
--
-- Phase 6 Wave 1, table 2-3 of 5: the unified rules engine. One rule binds
-- a trigger_event to a condition_set (recursive AND/OR/N_OF_M predicate
-- tree, stored as JSONB) and a list of effects. rule_evaluations is the
-- audit trail; predicate_state captures the snapshot used so a teacher can
-- ask "why didn't this fire?" weeks later.
--
-- Two enums introduced here are reused by 000034 (currency types) and
-- 000035 (accounts.tenant_mode):
--   gamification_scope_type — where a rule/currency lives in the org tree
--   gamification_audience   — pedagogical defaults: K-5/M68/H912/HigherEd/Corp/Pro

BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gamification_scope_type') THEN
        CREATE TYPE gamification_scope_type AS ENUM ('site','district','school','course','section');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gamification_audience') THEN
        CREATE TYPE gamification_audience AS ENUM ('k5','m68','h912','higher_ed','corp','pro');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS gamification_rules (
    id                bigserial PRIMARY KEY,
    tenant_id         bigint  NOT NULL,
    scope_type        gamification_scope_type NOT NULL,
    scope_id          bigint  NOT NULL,
    audience_level    gamification_audience   NOT NULL,
    name              text    NOT NULL,
    description       text,
    enabled           boolean NOT NULL DEFAULT TRUE,
    trigger_event     jsonb   NOT NULL,
    condition_set     jsonb   NOT NULL,
    effects           jsonb   NOT NULL,
    cooldown_seconds  int,
    max_per_window    jsonb,
    created_by        bigint,
    created_at        timestamptz NOT NULL DEFAULT NOW(),
    updated_at        timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gam_rules_scope
    ON gamification_rules (scope_type, scope_id) WHERE enabled;

CREATE INDEX IF NOT EXISTS idx_gam_rules_tenant
    ON gamification_rules (tenant_id) WHERE enabled;

CREATE TABLE IF NOT EXISTS gamification_rule_evaluations (
    id                   bigserial PRIMARY KEY,
    rule_id              bigint NOT NULL REFERENCES gamification_rules(id) ON DELETE CASCADE,
    user_id              bigint NOT NULL,
    evaluated_at         timestamptz NOT NULL DEFAULT NOW(),
    predicate_state      jsonb,
    result               boolean NOT NULL,
    effects_fired        jsonb,
    triggering_event_id  bigint REFERENCES gamification_events(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_gam_eval_user_rule_time
    ON gamification_rule_evaluations (user_id, rule_id, evaluated_at DESC);

CREATE INDEX IF NOT EXISTS idx_gam_eval_rule_time
    ON gamification_rule_evaluations (rule_id, evaluated_at DESC);

-- The (rule_id, user_id, evaluated_at) tuple is the natural snapshot key
-- per PHASE6-WAVE1-PLAN.md. The plan made it the primary key; we keep a
-- bigserial PK for clean GORM/repo ergonomics and surface the tuple here
-- as a UNIQUE so a same-microsecond duplicate evaluation surfaces as an
-- error rather than silently double-firing effects.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_gam_eval_rule_user_time
    ON gamification_rule_evaluations (rule_id, user_id, evaluated_at);

COMMIT;
