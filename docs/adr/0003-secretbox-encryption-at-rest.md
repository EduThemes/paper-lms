# 0003. `secretbox` envelope (AES-256-GCM + versioned key_id) for at-rest secrets

## Status

Accepted

## Context

Phase 9 introduced four classes of secrets that have to live in the
database: TOTP shared secrets, OIDC client secrets, LDAP bind
passwords (Sprint 10-C), and any future provider credential a tenant
admin configures through the app. Plaintext storage is non-negotiable:
a Postgres backup leak would compromise every tenant's IdP
configuration and every learner's MFA.

We need:

- Authenticated encryption (not just confidentiality) so a tampered
  ciphertext doesn't decrypt to garbage that the app then trusts.
- Per-record nonce so repeated values don't collide.
- A path to key rotation that doesn't require a flag day.
- Implementation simple enough to audit in a single sitting.

NaCl's `secretbox` is the model — short, opinionated, hard to misuse.
We don't use the library directly because we need a versioned key_id
header for rotation. AES-256-GCM is in Go's stdlib and meets the same
authenticated-encryption guarantees.

## Decision

**Every DB-resident secret round-trips through
`internal/auth/secretbox.go` before insert and after read.**

Envelope format:

```
[1 byte: key_id] [12 bytes: nonce] [N bytes: AES-256-GCM ciphertext+tag]
```

- `key_id` is a small integer that names which key version encrypted
  this record. New keys get the next id; the decrypt path looks up the
  right key for the prefix, so a rotation is "add the new key, encrypt
  new writes with it, leave old records alone."
- `nonce` is 12 bytes from `crypto/rand`. Per-record, never reused.
- Key material comes from the `MFA_ENCRYPTION_KEY` env var (32 bytes
  base64-encoded). The name is historical (Phase 9 / TOTP); the key
  is shared across every secret type.

Exported API:

- `secretbox.Encrypt(plaintext []byte) ([]byte, error)` — always uses
  the current key_id.
- `secretbox.Decrypt(ciphertext []byte) ([]byte, error)` — reads the
  key_id from the prefix.

**Rule:** any new column ending in `_encrypted` (or any secret column
regardless of name) MUST go through `secretbox.Encrypt` in the
repository layer. No plaintext secret columns ship past Phase 9.

In-flight callers (verified 2026-05-17):

- TOTP — `users.totp_secret_encrypted` (migration 000046).
- OIDC client secret — `authentication_providers` (Sprint 10-A.1).
- LDAP bind password — `authentication_providers.ldap_bind_password_encrypted`
  (backfilled via migration 000060).
- WebAuthn passkey ceremony state — encoded into the HttpOnly
  `passkey_ceremony` cookie via the same envelope (Sprint 10-B).

## Consequences

- **Boot-time key check**: `EnsureKeysLoaded` (Phase 12) validates
  `MFA_ENCRYPTION_KEY` is present and well-formed before the server
  accepts requests. `/ready` includes encryption-key health.
- **Key rotation is online**: add the new key to the keyring with the
  next id, restart, new writes use it, old records still decrypt. A
  background re-encrypt is optional, not required.
- **A leaked Postgres backup is degraded, not compromised**. Attackers
  still need `MFA_ENCRYPTION_KEY` (kept in the deployment's secret
  manager, not in the DB).
- **The global `~/.gitignore_global` excludes `*secret*`** on many dev
  machines, which silently drops `internal/auth/secretbox.go` from
  `git add`. The project `.gitignore` carries an explicit
  `!internal/auth/secretbox.go` negation. New "secret"-named source
  files need the same negation.
- **Filename `secretbox_test.go` covers the round-trip**; expand it
  when a new key id rotates in.

## References

- `internal/auth/secretbox.go` — the envelope implementation
- `internal/auth/secretbox_test.go` — round-trip + rotation tests
- `internal/auth/totp.go` — first consumer
- `internal/db/migrations/000046_*.up.sql` — first encrypted column
- `internal/db/migrations/000060_backfill_ldap_bind_password_encrypted.up.sql`
- `.gitignore` — the `!internal/auth/secretbox.go` negation
