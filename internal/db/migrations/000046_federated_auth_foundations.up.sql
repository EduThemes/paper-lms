-- 000046_federated_auth_foundations.up.sql
--
-- Phase 9 Sprint 9-PRE — auth foundations.
--
-- Purpose: stage the schema for OIDC client mode (9-A) + TOTP 2FA (9-B)
-- + WebAuthn passkeys (9-C, deferred) in ONE migration so the
-- refactor of SAML/LDAP/CAS through a shared LoginPipeline has every
-- table and column it needs from the start.
--
-- Five additions:
--   1. federated_identities — anchors external IdP subjects to local
--      user rows. Replaces the implicit "match by email" the existing
--      SAML/LDAP/CAS code does, which is the W3-B-style class-of-bug
--      we already lived through with pseudonym_name.
--   2. accounts.mfa_policy — per-tenant 2FA enforcement.
--   3. users.webauthn_user_handle — 64 random bytes per user, STABLE
--      forever. Required by 9-C passkeys; we generate it now (cheap)
--      so we don't pay a backfill migration later.
--   4. users.totp_secret_encrypted + users.totp_verified_at — TOTP
--      enrollment state. Ciphertext via internal/auth/secretbox
--      (AES-256-GCM); plaintext column would be a security finding
--      on day one.
--   5. user_recovery_codes — single-use bcrypt-hashed recovery codes
--      for users who lose their TOTP device.
--
-- Also:
--   * authentication_providers.ldap_bind_password_encrypted —
--     migrating away from plaintext LDAP bind passwords. The Go
--     server backfills existing rows on first boot (idempotent).
--   * authentication_providers.auto_provision — per-provider JIT
--     toggle. Default FALSE; first OIDC provider an admin configures
--     gets TRUE via repo-layer logic (per user decision 2026-05-15).

BEGIN;

-- pgcrypto required for gen_random_bytes() in the default expression.
-- Idempotent.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 1. Federated identity anchor.
CREATE TABLE IF NOT EXISTS federated_identities (
    id                bigserial PRIMARY KEY,
    user_id           bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_id       bigint      NOT NULL REFERENCES authentication_providers(id) ON DELETE CASCADE,
    external_subject  text        NOT NULL,
    claims_snapshot   jsonb,
    first_seen_at     timestamptz NOT NULL DEFAULT now(),
    last_seen_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_id, external_subject)
);
CREATE INDEX IF NOT EXISTS idx_federated_identities_user
    ON federated_identities (user_id);

-- 2. Per-tenant MFA policy.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS mfa_policy text NOT NULL DEFAULT 'off';

-- Add CHECK constraint via DO block so the migration is re-runnable.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'accounts_mfa_policy_check'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_mfa_policy_check
            CHECK (mfa_policy IN ('off','optional','required_admin','required_all'));
    END IF;
END$$;

-- 3 + 4. TOTP + passkey-ready user columns.
ALTER TABLE users
    -- 64 random bytes per user, generated at row-insert time. STABLE forever.
    -- 9-C passkeys will reference this; backfilling later would require a
    -- multi-step migration (add nullable → backfill → set NOT NULL).
    ADD COLUMN IF NOT EXISTS webauthn_user_handle bytea NOT NULL DEFAULT gen_random_bytes(64),
    -- TOTP secret as ciphertext (AES-256-GCM via internal/auth/secretbox).
    -- nullable: NULL = not enrolled.
    ADD COLUMN IF NOT EXISTS totp_secret_encrypted bytea,
    -- Set only after the user verifies a 6-digit code from their
    -- authenticator app. ENROLLMENT IS NOT FINAL UNTIL THIS IS SET,
    -- so a stolen session that "enrolls" but never verifies can't
    -- lock the real user out.
    ADD COLUMN IF NOT EXISTS totp_verified_at timestamptz;

-- 5. Single-use recovery codes; bcrypt-hashed.
CREATE TABLE IF NOT EXISTS user_recovery_codes (
    id          bigserial PRIMARY KEY,
    user_id     bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash   text        NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
-- Partial index over unused codes only — the typical lookup is "find
-- an unused code for this user during step-up." Used codes stay around
-- for audit but don't get indexed.
CREATE INDEX IF NOT EXISTS idx_user_recovery_codes_user_unused
    ON user_recovery_codes (user_id)
    WHERE used_at IS NULL;

-- Auth provider hardening:
--   * ldap_bind_password_encrypted: migrating the existing plaintext
--     ldap_bind_password column (still kept for one release, then
--     dropped in 000048 after the Go-side backfill confirms it ran).
--   * auto_provision: per-provider JIT toggle. The current
--     `jit_provisioning` field is repurposed at the same semantic
--     level; new code uses auto_provision. A future cleanup migration
--     can collapse the two.
ALTER TABLE authentication_providers
    ADD COLUMN IF NOT EXISTS ldap_bind_password_encrypted bytea,
    ADD COLUMN IF NOT EXISTS auto_provision boolean NOT NULL DEFAULT false;

COMMIT;
