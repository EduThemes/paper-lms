-- 000128_portfolio_share_token_digest.up.sql
--
-- ePortfolios (G155): portfolios.share_token held the private sharing
-- link's secret in plaintext. It now holds the SHA-256 digest (lower-case
-- hex) of the token; the link carries the token and lookups hash it. The
-- column stays a nullable text under the same partial unique index
-- (NULL = no link), so the *string model and index semantics are
-- unchanged.
--
-- Existing tokens (base64url, 32 chars) are hashed in place; a value that
-- is already a 64-char hex digest is left alone, so a re-run is a no-op.
-- Existing links keep working: the same token hashes to the stored value.

UPDATE portfolios
   SET share_token = encode(sha256(convert_to(share_token, 'UTF8')), 'hex')
 WHERE share_token IS NOT NULL
   AND share_token !~ '^[0-9a-f]{64}$';
