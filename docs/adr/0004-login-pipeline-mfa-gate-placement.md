# 0004. `LoginPipeline.Execute` as the single MFA-gate convergence

## Status

Accepted

## Context

By Phase 9 the auth surface had six credential types: local password,
SAML 2.0, LDAP, CAS 2.0, OIDC, and (Sprint 10-B) WebAuthn passkeys.
Each protocol had its own handler with its own version of "look up
the user by external subject, fall back to email auto-link, JIT
provision if the provider allows it." The triplicate was already a
maintenance burden, and the MFA policy that Phase 9-B introduced
(per-tenant `off` / `optional` / `required_admin` / `required_all`)
would have to be implemented six times — or, more realistically,
implemented in the local-password handler and silently skipped by
every SSO path.

The MFA gate is a security boundary. Six places to check it is six
places to forget.

## Decision

**Every credential type funnels through `auth.LoginPipeline.Execute`
after the handler verifies the credential. No new credential type
bypasses this.**

The handler's job ends at building an `SSOOutcome`
(`internal/auth/sso_outcome.go`). The pipeline owns everything after:

1. **`resolveUser`** — `federated_identities` lookup by
   `(provider_id, external_subject)` → email auto-link if
   `outcome.EmailVerified == true` → JIT provision if
   `outcome.ProviderConfig.AutoProvision`. Resolution priority is
   fixed; no protocol can reorder it.
2. **`decideMFAGate`** — reads the tenant's `accounts.mfa_policy`,
   checks the user's TOTP enrollment, returns one of `AllowSession`
   / `MustEnroll` / `MustVerify`.
3. **Outcome materialization** — mints a real session JWT for
   `AllowSession`, a 5-minute `purpose:"mfa_pending"` JWT for the
   verify/enroll cases. Both shapes are HTTP-agnostic; SAML / CAS
   set the cookie + redirect, the JSON endpoints return the token.
4. **Audit log** — every outcome emits `auth.login_succeeded` /
   `auth.login_failed` / `auth.mfa_required` with the provider type.

**Passkey is the explicit exception inside the pipeline, not outside
it.** A verified WebAuthn assertion is device possession + biometric/PIN
= two factors by definition. The pipeline checks
`outcome.ProviderType == "passkey"` and skips `decideMFAGate`, minting
the session directly. Do NOT layer TOTP on top of a passkey login —
that's not the locked UX.

`SSOOutcome` carries the contract:

- `ProviderID`, `ProviderType`, `ExternalSubject` (NameID / DN /
  principal / OIDC sub / decimal user_id for local).
- `Email`, `EmailVerified` — `EmailVerified=true` is the auto-link
  gate. SAML/LDAP/CAS = always true (IdP attested). OIDC = whatever
  the `email_verified` claim says (default false if absent). Local =
  true.
- `AttributesSnapshot` — the raw claim/attribute payload, written
  into `federated_identities.claims_snapshot` for Apple's
  first-login-only-claims quirk.

Provider-level `auto_provision`: the first provider an admin configures
for a tenant defaults to `true`; subsequent providers default `false`
and require explicit opt-in. The repo layer enforces the default;
admins toggle in the UI.

The Fiber `(result, wrote, err)` helper pattern is load-bearing inside
`Execute`: when a helper writes its own 4xx via `responses.Error`,
`err == nil` doesn't mean "happy path." The `wrote` flag is the abort
signal.

## Consequences

- **Adding a new credential type** is "build an `SSOOutcome`, hand it
  to `LoginPipeline.Execute`, done." No new user-resolution logic, no
  MFA-gate decisions, no audit-log plumbing.
- **The MFA policy is changeable in one place** when policy evolves
  (e.g., adding `required_students`, per-role rules, IP-based gates).
- **Passkey-skip is a single conditional**. Moving it (adding a new
  passwordless-but-still-MFA flow, say) is one change.
- **Pre-existing legacy JIT triplicate** (SAML/LDAP/CAS) was migrated
  through the pipeline in Sprint 10-C. The
  `grep -rn "FindByLoginID.*FindByEmail.*Create" internal/auth/` smoke
  test returns one match — the historical comment at the top of
  `login_pipeline.go`. No code path does inline JIT.
- **Verification harness**: `login_pipeline_test.go` is a 21-case
  matrix (local / OIDC / SAML / LDAP / CAS / passkey × user state ×
  policy) using in-test fakes rather than testify Mock so the matrix
  stays readable. Locks: passkey skips MFA; auto-link only on
  `EmailVerified=true`; `required_admin + admin + unenrolled → MustEnroll`.

## References

- `internal/auth/login_pipeline.go` — the convergence point
- `internal/auth/login_pipeline_test.go` — 21-case matrix
- `internal/auth/sso_outcome.go` — the contract
- `internal/auth/webauthn.go` — passkey-as-primary
- `internal/auth/mfa_pending.go` — separate JWT type for the gated state
- `internal/db/migrations/000046_*.up.sql` — `federated_identities`,
  `accounts.mfa_policy`, `users.webauthn_user_handle`
