-- 000059_wallet_transaction_idempotency.down.sql
--
-- Dropping the partial UNIQUE re-opens the TOCTOU race window in
-- ApplyTransaction. Existing duplicate-aware code paths will see
-- sql.ErrNoRows on a successful INSERT once the constraint is gone, so
-- the down migration is paired with a code rollback (revert ApplyTransaction
-- to a plain Create call). Operator decision: do not run this in
-- production without that paired rollback.

BEGIN;

DROP INDEX IF EXISTS uniq_wallet_tx_event_rule;

COMMIT;
