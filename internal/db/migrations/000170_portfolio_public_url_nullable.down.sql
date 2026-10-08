-- 000170_portfolio_public_url_nullable.down.sql
--
-- Intentionally no data change: turning NULLs back into '' would violate
-- idx_portfolios_public_url as soon as two unpublished portfolios exist.
SELECT 1;
