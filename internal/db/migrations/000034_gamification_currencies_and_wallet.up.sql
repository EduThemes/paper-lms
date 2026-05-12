-- 000034_gamification_currencies_and_wallet.up.sql
--
-- Phase 6 Wave 1, tables 4-5 of 5: user-defined currencies and the wallet
-- ledger. MyCred pattern — each tenant/course/section can define unlimited
-- currencies. Four are system-seeded on tenant creation (xp, gems,
-- mastery_points, reputation) via a Go-side seeder; not seeded here so
-- migrations stay declarative.
--
-- Balance rows reference currency_type by id (uint FK). Rules and rule
-- templates reference currencies by code (text) for portability — the
-- evaluator resolves code → id at apply time via the actor snapshot.

BEGIN;

CREATE TABLE IF NOT EXISTS gamification_currency_types (
    id                    bigserial PRIMARY KEY,
    tenant_id             bigint  NOT NULL,
    scope_type            gamification_scope_type NOT NULL,
    scope_id              bigint  NOT NULL,
    code                  text    NOT NULL,
    display_label         text    NOT NULL,
    display_label_plural  text,
    icon                  text,
    color                 text,
    display_order         int     NOT NULL DEFAULT 0,
    spendable             boolean NOT NULL DEFAULT FALSE,
    monotonic             boolean NOT NULL DEFAULT TRUE,
    ferpa_classification  text    NOT NULL DEFAULT 'non_PII'
        CHECK (ferpa_classification IN
            ('directory_information','education_record','non_PII','instructor_metadata')),
    max_balance           bigint,
    decay_policy          jsonb,
    visible_to_student    boolean NOT NULL DEFAULT TRUE,
    visible_in_topbar     boolean NOT NULL DEFAULT TRUE,
    system_owned          boolean NOT NULL DEFAULT FALSE,
    description           text,
    created_at            timestamptz NOT NULL DEFAULT NOW(),
    updated_at            timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT uniq_gam_currency_scope_code UNIQUE (tenant_id, scope_type, scope_id, code)
);

CREATE INDEX IF NOT EXISTS idx_gam_currency_tenant_scope
    ON gamification_currency_types (tenant_id, scope_type, scope_id);

CREATE INDEX IF NOT EXISTS idx_gam_currency_topbar
    ON gamification_currency_types (tenant_id, visible_in_topbar, display_order)
    WHERE visible_in_topbar;

CREATE TABLE IF NOT EXISTS gamification_wallet_balances (
    user_id           bigint NOT NULL,
    currency_type_id  bigint NOT NULL REFERENCES gamification_currency_types(id) ON DELETE RESTRICT,
    balance           bigint NOT NULL DEFAULT 0 CHECK (balance >= 0),
    lifetime_earned   bigint NOT NULL DEFAULT 0,
    updated_at        timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, currency_type_id)
);

CREATE INDEX IF NOT EXISTS idx_gam_wallet_balances_currency
    ON gamification_wallet_balances (currency_type_id);

CREATE TABLE IF NOT EXISTS gamification_wallet_transactions (
    id                   bigserial PRIMARY KEY,
    user_id              bigint NOT NULL,
    currency_type_id     bigint NOT NULL REFERENCES gamification_currency_types(id) ON DELETE RESTRICT,
    delta                bigint NOT NULL,
    reason               text   NOT NULL,
    triggering_event_id  bigint REFERENCES gamification_events(id) ON DELETE SET NULL,
    triggering_rule_id   bigint REFERENCES gamification_rules(id)  ON DELETE SET NULL,
    policy_flags         text[] NOT NULL DEFAULT '{}',
    occurred_at          timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gam_wallet_tx_user_time
    ON gamification_wallet_transactions (user_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_gam_wallet_tx_currency_time
    ON gamification_wallet_transactions (currency_type_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_gam_wallet_tx_rule
    ON gamification_wallet_transactions (triggering_rule_id)
    WHERE triggering_rule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_gam_wallet_tx_event
    ON gamification_wallet_transactions (triggering_event_id)
    WHERE triggering_event_id IS NOT NULL;

COMMIT;
