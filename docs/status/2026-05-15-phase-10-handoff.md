# Phase 10 handoff snapshot — 2026-05-15

This document is the fresh-session entry point. Read it, read `CLAUDE.md`, read `/Users/alfred/.claude/plans/joyful-fluttering-patterson.md`, then continue execution.

## Where we are

**Branch:** `phase6-wave3-prelude-logo` (~94 uncommitted files, ~12 sprints stacked).

**Working directory:** `/Users/alfred/Projects/Paper LMS/paper-LMS`

The branch name is leftover from the very first prelude sprint. It carries Wave 3 leaderboards → Phase 7 audit → Phase 8 polish → Phase 9 auth (OIDC + TOTP) → Phase 10-A hardening. Nothing has been split into per-sprint commits yet — the user has been reviewing in-place and will split before pushing.

## Running services (already up, NO need to restart)

| Service | Where | PID source | Notes |
|---|---|---|---|
| Postgres | localhost:5433 (Docker) | `paper-lms-postgres-1` | pgvector/pgvector:pg16; pgcrypto extension enabled |
| Backend | http://localhost:3000 | pid 7042 (`./bin/paper-lms`) | `AUTO_MIGRATE=false`; logs at `/tmp/paper-lms-server.log` |
| Frontend | http://localhost:5174 | pid 2466 (vite) | HMR active; service worker registers on first load |

Restart commands (only if something gets killed):
```bash
# Postgres
docker-compose up -d postgres

# Migrations (idempotent; current head: 000048)
make migrate-up

# Backend (in a separate shell, leaves running)
AUTO_MIGRATE=false ./bin/paper-lms > /tmp/paper-lms-server.log 2>&1 &

# Frontend
cd web && npm run dev > /tmp/paper-lms-frontend.log 2>&1 &
```

## Test accounts (password is `paperpaper` for all)

```
admin@paper.test          — seed admin
michael@aprendio.ai       — owner admin (note the 'e': aprENdio)
teacher@paper.test        — TeacherEnrollment
student1@paper.test       — Sofia Alvarez,        640 XP, rank 1
student2@paper.test       — Ben Carter,           520 XP, rank 2
student3@paper.test       — Chen Wei,             440 XP, rank 3
student4@paper.test       — Diego Martinez,       360 XP   ← opted out of leaderboard (W2-C filter test)
student5@paper.test       — Emma Patel,           250 XP, rank 4 (Diego skipped)
student6@paper.test       — Farah Khalil,         140 XP, rank 5
student7@paper.test       — Gabriel O'Donnell,    40 XP,  rank 6 (last — relative window + fillers fire)
```

Tenant on account_id=1 is `tenant_mode = 'k5'` so K-5 mechanics fire by default (pseudonyms, relative window, no top-N for students). `accounts.mfa_policy = 'off'` (login works without 2FA).

Seed re-runs: `DATABASE_URL=postgres://paper:paper@localhost:5433/paper_lms?sslmode=disable go run ./cmd/seedtestdata` (idempotent).

## What's done in this session (Phase 10-A: 5 of 8 items)

Source of truth for the full plan is `/Users/alfred/.claude/plans/joyful-fluttering-patterson.md`.

| Item | File(s) | Tests |
|---|---|---|
| A.2 OIDC login-page buttons + Apple style + handleSSOLogin routing fix | `web/src/pages/LoginPageSSO.jsx` | manual |
| A.3 `qrcode.react` inline QR | `web/src/pages/MFAEnrollPage.jsx`, `web/package.json` | manual |
| A.4 MFA brute-force rate limit | `internal/auth/mfa_rate_limit.go` + `_test.go`, `internal/api/v1/handlers/mfa.go`, `cmd/server/main.go` | 5 unit tests passing |
| A.5 TOTP code-reuse guard (RFC 6238 §5.2) | migration 000048, `internal/auth/totp.go`, `internal/auth/totp_reuse_test.go`, `internal/api/v1/handlers/mfa.go`, `internal/domain/models/user.go` | 3 unit tests passing |
| A.1 backend half (OIDC client_secret encryption on Create) | `internal/api/v1/handlers/auth_providers.go` | manual |

**Verification command before continuing:**
```bash
cd /Users/alfred/Projects/Paper\ LMS/paper-LMS
go build ./... && go vet ./... && go test ./internal/auth/... ./internal/api/v1/handlers/... ./internal/service/gamification/...
```
Expected: all green.

## What's next (Phase 10 remaining work)

Three groups, ordered by the locked plan:

### Sprint 10-A remaining (3 of 8 items)

These are deferred but the plan describes each in detail. They can land in any order; none blocks 10-B/10-C.

- **A.1 OIDC admin form UI** — `web/src/pages/admin/auth/OIDCProviderForm.jsx`. Four-preset wizard (Google, Microsoft, Apple, Generic). Backend already encrypts client_secret. ~150 LOC JSX.
- **A.6 LoginPipeline integration test matrix** — `internal/auth/login_pipeline_test.go`. ~25 test cases across (ProviderType × user state × email_verified × auto_provision × mfa_policy × enrolled). Needs `mocks.MockAuthenticationProviderRepository` added to `internal/testutil/mocks/repository_mocks.go`.
- **A.7 Dex docker-compose service** — `docker-compose.yml` + `test/dex/config.yaml` + `test/dex/seed.sh` + `internal/auth/oidc_integration_test.go`. Lets OIDC be E2E-tested without real Google/Microsoft credentials.
- **A.8 Real-IdP setup docs** — `docs/auth/oidc-providers.md`. Walkthroughs for Google Workspace, Microsoft Entra ID, Apple Sign-In, Authentik, Authelia, Keycloak, Zitadel.

### Sprint 10-B — Passkeys (WebAuthn) — full sprint

Locked UX: **passkey-as-primary** (replaces password; device biometric is the second factor by definition).

Foundations already in place from Phase 9-PRE:
- `users.webauthn_user_handle` (64 random bytes per user, stable forever).
- `LoginPipeline.Execute` accepts `SSOOutcome{ProviderType:"passkey"}`.
- Audit log event types ready.

What 10-B adds:
- Library: `github.com/go-webauthn/webauthn`.
- Migration 000049: `user_webauthn_credentials` (credential_id UNIQUE, public_key_cose, sign_count, aaguid, transports, nickname, backup_eligible, backup_state, last_used_at).
- `internal/auth/webauthn.go` — Begin/FinishRegistration + Begin/FinishLogin. Ceremony state encrypted via `secretbox.Encrypt` into a 60s HttpOnly cookie (no Redis dependency).
- Routes: `POST/DELETE /users/self/passkeys/*` (4 routes, authenticated), `POST /auth/passkey/{begin,finish}` (2 routes, anonymous).
- Pipeline branch: `ProviderType=="passkey"` skips email auto-link AND the MFA gate.
- Frontend: `PasskeyEnrollPage` (mirrors `MFAEnrollPage`), `PasskeyListPage`, "Sign in with a passkey" button on `LoginPageSSO`.

Plan section: "Sprint 10-B — Passkeys (WebAuthn)" in `/Users/alfred/.claude/plans/joyful-fluttering-patterson.md`.

### Sprint 10-C — SAML/LDAP/CAS refactor through pipeline

Locked depth: **full rewrite** (not thin wrapper).

What stays untouched:
- SAML XML signature verification (`verifyResponseSignature`, `verifyAssertionSignature`, certificate-fingerprint matching). ~400 LOC.
- LDAP bind protocol. ~300 LOC.
- CAS ticket validation XML parsing. ~200 LOC.

What changes (~50 LOC per handler):
- Each handler emits `SSOOutcome{...}` instead of doing inline JIT.
- `sso_handler.go` is wired with `*auth.LoginPipeline` and passes it through.
- Existing tests for signature/bind/ticket parsing must stay byte-for-byte identical — a diff that touches those is a red flag.
- ~20 test cases in the handler suites get updated to assert on pipeline-call behavior instead of inline-JIT behavior.

Validation: `grep -rn "FindByLoginID.*FindByEmail.*Create" internal/auth/` must return zero matches after the refactor. That's the load-bearing assertion that the JIT triplicate is gone.

## Key files the fresh session should know

**Plan + status (read these first):**
- `/Users/alfred/.claude/plans/joyful-fluttering-patterson.md` — the Phase 10 plan; the source of truth for sprint scope.
- `CLAUDE.md` — load-bearing patterns. Sections "Phase 7 patterns," "Phase 9 / 10 patterns" have the load-bearing conventions.
- `docs/audits/2026-05-15-gamification-audit.md` — Phase 7 audit findings + remediation log.

**Code that 10-B / 10-C will touch:**
- `internal/auth/login_pipeline.go` — pipeline; needs a `ProviderType=="passkey"` branch for 10-B and `case "saml" | "ldap" | "cas":` branches that no-op (the existing handlers will provide outcomes) for 10-C.
- `internal/auth/sso_outcome.go` — the convergence shape.
- `internal/auth/secretbox.go` — encryption-at-rest wrapper for any new secrets (passkey ceremony state).
- `internal/auth/auth_audit.go` — typed audit events; 10-B adds `passkey_registered`, `passkey_used`.
- `internal/api/v1/handlers/auth_providers.go` — extend for OIDC UpdateProvider client_secret encryption.
- `internal/auth/{saml,ldap,cas,sso_handler}.go` — 10-C refactor targets.

**Critical files NOT to touch in 10-B/10-C (signature verification + crypto):**
- `internal/auth/saml.go` — `verifyResponseSignature` and friends, lines containing XML signature work.
- `internal/auth/ldap.go` — `tls.Dial` + bind protocol.
- `internal/auth/cas.go` — `serviceValidate` XML parsing.

## Conventions the next session needs to follow

(Already documented in CLAUDE.md but worth surfacing:)

- **No `Co-Authored-By: Claude` lines in commits.** EduThemes/paper-lms org policy.
- **iCloud-dupe sweep before commit:** `find . -name "* 2.go" -o -name "* 2.sql"` should be empty.
- **Bool-default lesson:** no `default:` GORM tag on policy-bearing bools — `db.Save` on Update, raw `INSERT ... RETURNING` on Create.
- **`*string` for nullable text on policy-bearing partial UNIQUE indexes** — GORM serializes `string` as `''` not `NULL`.
- **`secretbox.Encrypt` for ALL new at-rest secrets.** No plaintext secret columns ship.
- **Every credential type funnels through `LoginPipeline.Execute`** — no exceptions in new code.
- **MFA pending tokens are a separate JWT type** (`purpose:"mfa_pending"`, 5-min TTL). Not a flag on the regular session.

## Decision log from this session (locked)

These came up in planning conversations; record so the next session doesn't relitigate.

- **Phase 9 — MFA policy granularity:** per-tenant only (`off`/`optional`/`required_admin`/`required_all`).
- **Phase 9 — OIDC JIT default:** per-provider toggle; ON for the first provider an admin configures, OFF for subsequent.
- **Phase 9 — Federated email match:** auto-link only when `email_verified=true` from the IdP.
- **Phase 9 — OIDC presets:** Google, Microsoft, Apple, Generic.
- **Phase 10 — Sprint order:** Polish/hardening first (10-A), then passkeys (10-B), then legacy refactor (10-C). Lowest-risk → highest-risk.
- **Phase 10 — Legacy refactor depth:** full rewrite through pipeline (not thin wrapper or hybrid). Avoids the JIT-triplicate tech debt the user explicitly rejected in Phase 9.
- **Phase 10 — OIDC E2E:** both — Dex docker-compose for automated tests AND real-IdP setup docs.
- **Phase 10 — Passkey UX:** passkey-as-primary (replaces password; device biometric is the second factor by definition). Industry direction.

## How to start

Recommended fresh-session prompt:

> Continue Phase 10 work on Paper LMS. Read `docs/status/2026-05-15-phase-10-handoff.md` for the current state. The full plan is at `/Users/alfred/.claude/plans/joyful-fluttering-patterson.md`. Confirm the build + tests are green, then start on Sprint 10-B (passkeys). Run without stopping for clarifying questions — make the reasonable call and continue; I'll redirect if needed.

(Or substitute "Sprint 10-B" with whichever item you want next.)
