-- 000160_login_session_tokens.up.sql
--
-- G136: Canvas GET /login/session_token. A caller holding an API access
-- token may mint a short-lived, single-use URL that signs the same user
-- into a browser session. Only the SHA-256 of the token is stored, and
-- it is consumed with DELETE ... RETURNING.

BEGIN;

CREATE TABLE IF NOT EXISTS login_session_tokens (
    id bigserial,
    token_hash text NOT NULL,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id bigint NOT NULL,
    return_to text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_login_session_tokens_expires_at ON public.login_session_tokens USING btree (expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_login_session_tokens_token_hash ON public.login_session_tokens USING btree (token_hash);
CREATE INDEX IF NOT EXISTS idx_login_session_tokens_user_id ON public.login_session_tokens USING btree (user_id);

COMMIT;
