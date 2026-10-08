-- 000169_portfolio_sharing.up.sql
--
-- ePortfolios (G155), Canvas parity:
--   * share_token — a private sharing link. Canvas lets the owner share a
--     private ePortfolio through a secret link (eportfolios.uuid verifier)
--     without making it public. NULL = no link; the partial unique index
--     skips NULL rows, so the column is a nullable text (*string model).
--   * comments_disabled — the owner can turn viewer comments off (Canvas
--     entries carry allow_comments). Stored inverted so the GORM zero value
--     (false) is the default and GORM never omits a meaningful value.

ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS share_token text;
CREATE UNIQUE INDEX IF NOT EXISTS idx_portfolios_share_token
    ON portfolios (share_token) WHERE share_token IS NOT NULL;

ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS comments_disabled boolean NOT NULL DEFAULT false;
