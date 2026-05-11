-- Pairing codes for parent/observer linking (Canvas-compatible).
CREATE TABLE IF NOT EXISTS pairing_codes (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    user_id     BIGINT      NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    redeemed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pairing_codes_code
    ON pairing_codes (code);

CREATE INDEX IF NOT EXISTS idx_pairing_codes_user_id
    ON pairing_codes (user_id);

CREATE INDEX IF NOT EXISTS idx_pairing_codes_expires_at
    ON pairing_codes (expires_at);
