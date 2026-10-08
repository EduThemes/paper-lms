-- 000134_calendar_feed_tokens.up.sql
--
-- Calendar parity (G096): a per-user secret behind the subscribable ICS
-- feed URL. token_digest (SHA-256 hex) is the lookup key; the token
-- itself is only stored sealed with secretbox (token_ciphertext) so the
-- calendar page can show the URL again. One token per user; rotating it
-- revokes the old URL.

CREATE TABLE IF NOT EXISTS calendar_feed_tokens (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_digest text NOT NULL,
    token_ciphertext bytea,
    last_accessed_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cal_feed_token_user ON calendar_feed_tokens (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cal_feed_token_digest ON calendar_feed_tokens (token_digest);
