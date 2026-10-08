-- 000169_portfolio_sharing.down.sql

ALTER TABLE portfolios DROP COLUMN IF EXISTS comments_disabled;
DROP INDEX IF EXISTS idx_portfolios_share_token;
ALTER TABLE portfolios DROP COLUMN IF EXISTS share_token;
