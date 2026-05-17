-- 000057_settings.up.sql
--
-- Super-Admin Settings Engine, Wave 1 — schema stage.
--
-- A generic key-value store for operational config (SMTP, S3, OIDC,
-- Anthropic API key, etc.) that currently lives only in environment
-- variables. The settings service walks the scope chain
-- (user → account-parent-chain → instance → env → default) and resolves
-- the highest-priority value; this table is the storage backing for
-- everything above "env".
--
-- value_plain holds non-secret values (e.g. SMTP_HOST, S3_REGION).
-- value_encrypted holds AES-256-GCM ciphertext produced by
-- internal/auth/secretbox (1-byte key_id + 12-byte nonce + ciphertext+tag).
-- Exactly one of the two is non-NULL per row; the value_type column
-- declares which side carries the value and what the catalog should
-- coerce on read.
--
-- Why a single generic table rather than per-domain tables: every
-- env-var promotion would otherwise require a schema migration. The
-- typed reader layer lives in internal/service/settings/catalog.go
-- where each known key is declared with its ValueType, Scopes,
-- EnvFallback, and Default.
--
-- Resolution is index-bound by (scope_type, scope_id, key); the UNIQUE
-- constraint also doubles as the natural-key collision guard for the
-- upsert path.

BEGIN;

CREATE TABLE IF NOT EXISTS settings (
    id              BIGSERIAL PRIMARY KEY,
    scope_type      TEXT NOT NULL,
    scope_id        BIGINT NOT NULL DEFAULT 0,
    key             TEXT NOT NULL,
    value_plain     TEXT,
    value_encrypted BYTEA,
    value_type      TEXT NOT NULL DEFAULT 'string',
    updated_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT settings_scope_type_check
        CHECK (scope_type IN ('instance', 'account', 'user')),
    CONSTRAINT settings_value_type_check
        CHECK (value_type IN ('string', 'int', 'bool', 'json', 'secret')),
    CONSTRAINT settings_exactly_one_value
        CHECK (
            (value_plain IS NOT NULL AND value_encrypted IS NULL)
         OR (value_plain IS NULL AND value_encrypted IS NOT NULL)
        ),
    CONSTRAINT settings_secret_uses_encrypted
        CHECK (value_type <> 'secret' OR value_encrypted IS NOT NULL),
    CONSTRAINT settings_scope_unique UNIQUE (scope_type, scope_id, key)
);

CREATE INDEX IF NOT EXISTS idx_settings_lookup
    ON settings (scope_type, scope_id, key);

COMMIT;
