-- 000059_wallet_transaction_idempotency.up.sql
--
-- Closes a TOCTOU race between CheckCooldown and ApplyTransaction in the
-- gamification dispatcher. The cooldown gate issues a plain SELECT against
-- gamification_rule_evaluations and then — in a separate transaction — runs
-- effects which ledger currency via wallet ApplyTransaction. Two concurrent
-- emits for the same (user, rule) can both see "no prior firing", both pass
-- the gate, and both append wallet transactions. The wallet's row-level
-- balance lock (gamification_wallet.go:55-107) serializes WRITES but does
-- not deduplicate them — both deltas accumulate against the balance.
--
-- The pre-existing UNIQUE index uniq_gam_eval_rule_user_time on
-- (rule_id, user_id, evaluated_at) only catches same-microsecond
-- duplicates, which is useless for the realistic millisecond-wide race
-- window between two HTTP workers.
--
-- The fix here is to make the wallet ledger itself idempotent on the
-- triggering pair (event_id, rule_id). A wallet transaction is the
-- materialization of "rule R fired in response to event E for user U".
-- That tuple is uniquely identifying — re-emitting it should be a no-op,
-- not a second award. ApplyTransaction will issue
-- `INSERT ... ON CONFLICT DO NOTHING RETURNING id` and translate
-- sql.ErrNoRows into a typed repository.ErrDuplicateWalletTransaction
-- sentinel (same pattern as gamification_currency_types Create at
-- internal/repository/postgres/gamification_currency_type.go:41-68).
--
-- Partial: triggering_event_id IS NULL means the transaction was not
-- driven by an event (manual admin grant, seed, spend) — those rows
-- intentionally bypass the idempotency check because their natural key
-- lives elsewhere (manual:<actor_id>, spend:<sku>, seed:<source>).
--
-- Out of scope: the rule_evaluations INSERT shares the same TOCTOU
-- shape, but its purpose is audit-trail truthiness (every consideration
-- is recorded, not just every firing). That's a separate follow-up.
--
-- The index is idempotent (IF NOT EXISTS) so re-applying the migration
-- on a hot-fixed environment doesn't fail.

BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_wallet_tx_event_rule
    ON gamification_wallet_transactions (triggering_event_id, triggering_rule_id)
    WHERE triggering_event_id IS NOT NULL;

COMMIT;
