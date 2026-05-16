-- 000050_gamification_enums_and_fks.up.sql
--
-- Phase 7-A backlog closeout. Lands two audit follow-ups in one
-- migration:
--
--   * F1.2 — FK-by-convention across gamification tables. The xAPI
--     event substrate (and downstream wallet / badge ledgers)
--     reference users.id and accounts.id by integer convention; a
--     stray DELETE on either parent leaves orphans the rules engine
--     happily processes. Add real FKs with appropriate ON DELETE
--     semantics so the cascade story matches the deletion story.
--
--   * F1.4 follow-up — gamification_badges.scope_type was hardened to
--     a CHECK constraint in 000044; the audit's eventual fix was to
--     cast to the gamification_scope_type enum (same shape as
--     gamification_currency_types.scope_type and
--     gamification_rules.scope_type). Do that now: drop the CHECK,
--     cast the column, restore the partial UNIQUE if any read path
--     depended on its ordering. The cast is safe because the CHECK
--     constraint has prevented any out-of-range writes since 7-A
--     close — the DB shape is already a subset of the enum.
--
--   * F1.11 — gamification_badges.audience_level was relaxed to
--     nullable text in 000044. Align to the gamification_audience
--     enum (same enum as gamification_rules.audience_level). Cast is
--     safe via NULL-tolerant USING because the column was already
--     nullable and existing non-NULL values are either '' (becomes
--     NULL by the WHERE clause below) or one of the six valid enum
--     literals.
--
-- All work is wrapped in a single BEGIN/COMMIT so a partial migration
-- doesn't strand the schema in an inconsistent state.

BEGIN;

-- ---- F1.4 follow-up: badges.scope_type → enum ---------------------------

ALTER TABLE gamification_badges
    DROP CONSTRAINT IF EXISTS chk_gam_badges_scope_type;

ALTER TABLE gamification_badges
    ALTER COLUMN scope_type TYPE gamification_scope_type
    USING scope_type::gamification_scope_type;

-- ---- F1.11: badges.audience_level → enum -------------------------------

-- Clear any leftover empty-string values (the column was NOT NULL
-- DEFAULT '' pre-000044; nullable post-000044 but stranded rows may
-- still carry '' if they predate the relaxation).
UPDATE gamification_badges
SET audience_level = NULL
WHERE audience_level = '';

ALTER TABLE gamification_badges
    ALTER COLUMN audience_level TYPE gamification_audience
    USING NULLIF(audience_level, '')::gamification_audience;

-- ---- F1.2: FKs on user_id / tenant_id columns --------------------------

-- gamification_events.tenant_id → accounts(id)
--   The event store cascades on tenant delete: a tenant going away
--   means every event for that tenant is meaningless. Wallet
--   transactions and badge awards keyed off those events also need to
--   cascade — they already do via existing FKs to events / currencies.
ALTER TABLE gamification_events
    ADD CONSTRAINT fk_gam_events_tenant
    FOREIGN KEY (tenant_id) REFERENCES accounts(id) ON DELETE CASCADE;

-- gamification_events.actor_id → users(id)
--   actor_id is the xAPI "Actor" of the event. CASCADE on user delete
--   so the event history vanishes with the user (FERPA + GDPR-friendly).
ALTER TABLE gamification_events
    ADD CONSTRAINT fk_gam_events_actor
    FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE CASCADE;

-- gamification_currency_types.tenant_id → accounts(id)
ALTER TABLE gamification_currency_types
    ADD CONSTRAINT fk_gam_currencies_tenant
    FOREIGN KEY (tenant_id) REFERENCES accounts(id) ON DELETE CASCADE;

-- gamification_wallet_balances.user_id → users(id)
--   Hard ownership: a user delete removes the balance row.
ALTER TABLE gamification_wallet_balances
    ADD CONSTRAINT fk_gam_wallet_balances_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- gamification_wallet_transactions.user_id → users(id)
ALTER TABLE gamification_wallet_transactions
    ADD CONSTRAINT fk_gam_wallet_tx_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- gamification_rules.tenant_id → accounts(id)
ALTER TABLE gamification_rules
    ADD CONSTRAINT fk_gam_rules_tenant
    FOREIGN KEY (tenant_id) REFERENCES accounts(id) ON DELETE CASCADE;

-- gamification_rules.created_by → users(id)
--   Author attribution survives the author leaving — SET NULL, don't
--   delete the rule.
ALTER TABLE gamification_rules
    ADD CONSTRAINT fk_gam_rules_created_by
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

-- gamification_rule_evaluations.user_id → users(id)
--   Evaluation history is per-user; cascade on user delete.
ALTER TABLE gamification_rule_evaluations
    ADD CONSTRAINT fk_gam_rule_evals_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- gamification_badges.tenant_id → accounts(id)
ALTER TABLE gamification_badges
    ADD CONSTRAINT fk_gam_badges_tenant
    FOREIGN KEY (tenant_id) REFERENCES accounts(id) ON DELETE CASCADE;

-- gamification_badges.created_by → users(id), SET NULL on author delete.
ALTER TABLE gamification_badges
    ADD CONSTRAINT fk_gam_badges_created_by
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

-- gamification_badge_awards.user_id → users(id)
ALTER TABLE gamification_badge_awards
    ADD CONSTRAINT fk_gam_badge_awards_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- gamification_badge_awards.awarded_by → users(id)
--   Optional manual-grant attribution — SET NULL on awarder delete.
ALTER TABLE gamification_badge_awards
    ADD CONSTRAINT fk_gam_badge_awards_awarded_by
    FOREIGN KEY (awarded_by) REFERENCES users(id) ON DELETE SET NULL;

-- gamification_leaderboard_snapshots.tenant_id → accounts(id)
ALTER TABLE gamification_leaderboard_snapshots
    ADD CONSTRAINT fk_gam_snapshots_tenant
    FOREIGN KEY (tenant_id) REFERENCES accounts(id) ON DELETE CASCADE;

-- Note on scope_id: deliberately NOT FK'd. The (scope_type, scope_id)
-- pair is polymorphic by design (F1.3 verdict — defensible
-- architectural choice; the scope walk needs uniform shape). A future
-- migration could enforce per-scope_type validity via trigger or
-- partial-index magic; not in scope for v1.

COMMIT;
