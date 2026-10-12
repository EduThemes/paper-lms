-- 000127_portfolio_public_url_nullable.up.sql
--
-- ePortfolios (G155): portfolios.public_url has a UNIQUE index but every
-- unpublished portfolio stored '' in it, so a user's second portfolio (or a
-- second "Duplicate") failed with a duplicate-key error. Unpublished
-- portfolios now store NULL, which the unique index treats as distinct.

UPDATE portfolios SET public_url = NULL WHERE public_url = '';
