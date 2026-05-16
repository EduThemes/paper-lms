-- 000049_user_webauthn_credentials.up.sql
--
-- Phase 10 Sprint 10-B — WebAuthn / passkey credentials.
--
-- One row per registered passkey per user. A user can have several
-- (laptop Touch ID + phone Face ID + a hardware security key) — the
-- list page lets them name and revoke each.
--
-- credential_id is the WebAuthn-spec-defined opaque identifier
-- returned by the authenticator at registration. It is what comes
-- back on every subsequent assertion; we use it to look up the row.
-- UNIQUE because credential_ids are globally unique by spec.
--
-- public_key_cose is the COSE-encoded public key. The go-webauthn
-- library decodes it for assertion verification.
--
-- sign_count is the authenticator's monotonically-increasing counter.
-- A drop or stall in this counter signals a cloned authenticator —
-- the library rejects the assertion when this happens.
--
-- backup_eligible / backup_state: whether the authenticator advertises
-- itself as cloud-syncable (iCloud Keychain, Google Password Manager,
-- 1Password) and whether the credential is currently synced. v1 stores
-- these for forensics + future UI badges ("synced" indicator on the
-- list page); the assertion path does not gate on them.
--
-- users.webauthn_user_handle is already present from migration 000046,
-- so no users-table changes are needed for this sprint.

BEGIN;

CREATE TABLE IF NOT EXISTS user_webauthn_credentials (
    id              bigserial   PRIMARY KEY,
    user_id         bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id   bytea       NOT NULL UNIQUE,
    public_key_cose bytea       NOT NULL,
    sign_count      bigint      NOT NULL DEFAULT 0,
    aaguid          bytea,
    transports      text[],
    nickname        text,
    backup_eligible boolean     NOT NULL DEFAULT false,
    backup_state    boolean     NOT NULL DEFAULT false,
    last_used_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_webauthn_credentials_user
    ON user_webauthn_credentials (user_id);

COMMIT;
