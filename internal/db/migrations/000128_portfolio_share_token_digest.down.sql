-- 000128_portfolio_share_token_digest.down.sql
--
-- A digest cannot be turned back into its token, and the pre-000128 code
-- compares the column to the token as plaintext, so no stored link could
-- match. Clear them: owners create new sharing links after a rollback.

UPDATE portfolios SET share_token = NULL WHERE share_token IS NOT NULL;
