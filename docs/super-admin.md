# Super-Admin Settings Engine

**Status:** Wave 2 (read-only API) shipped. Wave 3 (writes + test actions)
and Wave 4 (frontend panel) are next.

The Super-Admin Settings Engine lets a platform operator manage the
operational config that used to require an engineer editing `.env` and
restarting the server — SMTP creds, S3 bucket, OIDC redirect base,
Anthropic API key, branding URL, and so on. Districts running on a
single Hetzner box or homelab Docker can now provision the operator
once at first boot and self-service the rest.

This doc covers the **super_admin role** and the **Wave 2 read API**.

---

## The `super_admin` role

`super_admin` is a third value of `users.role`, alongside `user` and
`admin` (migration 000058 widens the CHECK constraint). It is strictly
above account-admin:

| Capability                                       | `user` | `admin` | `super_admin` |
|--------------------------------------------------|:------:|:-------:|:-------------:|
| Manage own profile                               |   ✓    |    ✓    |       ✓       |
| Manage their tenant's accounts/courses/users     |        |    ✓    |       ✓       |
| Read/write `/superadmin/settings/*`              |        |         |       ✓       |
| Cross-tenant inspection (any `account_id`)       |        |         |       ✓       |

Critically, `super_admin` is **NOT** "an admin of account 1." An admin
of the root account does **not** inherit super-admin access. The role
is checked as a literal string match (`role == "super_admin"`); case
variations and trimmed whitespace are rejected.

This contract is asserted in tests at
`internal/api/v1/handlers/super_admin_isolation_test.go` — the
"`TestSuperAdminGate_RootAccountAdminIs403`" and
"`TestSuperAdminGate_CompromisedRoleLiteralRejected`" cases are
specifically the Canvas-LMS-CVE-2021-32585-class regressions we
guard against.

---

## Bootstrap (first deploy)

On a fresh deployment the first user created via the setup wizard
(`POST /api/v1/setup/complete`) is promoted to `super_admin`
automatically. Subsequent platform operators are promoted by an
existing `super_admin`. There is no "self-promotion" path: the
account-admin `PUT /users/:id/role` endpoint accepts
`user|admin|teacher|observer` and rejects `super_admin`.

**Existing deployments** (those that already completed setup before
the Settings Engine landed) keep their existing `admin` users
unchanged. There is no auto-promotion. An operator with shell access
to the DB can promote a chosen user manually:

```sql
-- Promote a specific user to super_admin. Pick someone real; the
-- CHECK constraint enforces vocabulary at the DB layer.
UPDATE users SET role = 'super_admin' WHERE email = 'ops@example.com';
```

A CLI helper for this lands with Wave 3.

---

## Wave 2 — Read API

All routes sit behind `Protected()` AND `RequireSuperAdmin()`. They are
mounted exclusively via `registerSuperAdminRoutes` in
`internal/api/v1/routes_super_admin.go` so the gate-pair cannot be
silently dropped on a new endpoint.

### `GET /api/v1/superadmin/settings[?account_id=N]`

Returns every catalog entry with its resolved effective value.
Secrets are **always** masked — the response carries `is_secret: true`
+ `has_value: true|false` but never the plaintext value.

The optional `?account_id=N` query parameter sets the account hint
for the resolution chain. A super-admin reading without `?account_id`
sees the instance-scope + env + default fallback chain only; passing
`?account_id=42` projects what a caller in account 42 would see
(walks the parent chain from 42 up to root, then falls through to
instance/env/default).

Sample response:

```json
{
  "settings": [
    {
      "key": "smtp.host",
      "group": "Email",
      "label": "SMTP host",
      "value_type": "string",
      "is_secret": false,
      "source": "instance",
      "has_value": true,
      "value": "mail.example.test",
      "scope_id": 0,
      "updated_at": "2026-05-17T12:34:56Z",
      "updated_by": 1
    },
    {
      "key": "smtp.password",
      "group": "Email",
      "label": "SMTP password",
      "value_type": "secret",
      "is_secret": true,
      "source": "instance",
      "has_value": true,
      "scope_id": 0,
      "updated_at": "2026-05-17T12:34:56Z",
      "updated_by": 1
    },
    {
      "key": "smtp.port",
      "group": "Email",
      "label": "SMTP port",
      "value_type": "int",
      "is_secret": false,
      "source": "default",
      "has_value": true,
      "value": "587"
    }
  ]
}
```

`source` is one of `user|account|instance|env|default|none`. The UI
should render `source=env` as a read-only field with a hint "Configured
via environment variable — clear to override here" once Wave 3 enables
writes.

### `GET /api/v1/superadmin/settings/:key[?account_id=N]`

Single-key variant of the above. Returns 404 for keys not declared in
the catalog (`internal/service/settings/catalog.go`) — the API surface
intentionally does not enumerate "what we don't recognize."

### `GET /api/v1/superadmin/settings/groups`

Returns the **catalog vocabulary** — Definition only, no live values.
The frontend uses this to build the settings form. Live values come
from the two endpoints above.

```json
{
  "definitions": [
    {
      "key": "smtp.host",
      "group": "Email",
      "label": "SMTP host",
      "description": "Hostname of the outbound SMTP server.",
      "value_type": "string",
      "is_secret": false,
      "scopes": ["instance", "account"],
      "env_fallback": "SMTP_HOST",
      "has_default": false,
      "test_action": "email"
    }
  ]
}
```

The vocabulary endpoint and the live-value endpoints serve **different
JSON shapes** by design. A single response that mixed "what could be
set" with "what currently is" would invite the kind of UI bug that
leaks an unmask through a debug serialization.

---

## Manual smoke test

```bash
# Replace with your real domain. Cookie value comes from the session
# created by a normal browser login as the super_admin.
SESSION='paper_session=eyJhbGc...'
BASE='https://paper.example.org/api/v1'

# Sanity-check role:
curl -sH "Cookie: $SESSION" "$BASE/users/self" | jq '.role'
# → "super_admin"

# Pull the vocabulary:
curl -sH "Cookie: $SESSION" "$BASE/superadmin/settings/groups" | jq

# Pull all live settings:
curl -sH "Cookie: $SESSION" "$BASE/superadmin/settings" | jq '.settings[] | {key, source, has_value, is_secret}'

# Project what tenant 42 would see:
curl -sH "Cookie: $SESSION" "$BASE/superadmin/settings?account_id=42" | jq '.settings[] | select(.source=="account")'
```

If a request returns 403, verify your session belongs to a user with
`role = 'super_admin'` (not `'admin'`).

---

## What's NOT in Wave 2

- **No writes.** `PUT/DELETE` on `/superadmin/settings/:key` lands in Wave 3.
- **No test actions.** The `test_action` field in the vocabulary is
  metadata for the UI; the actual `POST /superadmin/settings/test/...`
  endpoints land with writes in Wave 3.
- **No frontend panel.** The Wave 2 surface is curl-friendly; Wave 4
  ships the React panel at `/superadmin/settings`.
- **No FERPA export of account-scoped settings.** Wave 5 follow-up.

---

## Security model

See the header comment of
`internal/api/v1/handlers/super_admin_settings.go` for the locked
threat model. The short version:

1. Every route mounted by `registerSuperAdminRoutes` inherits
   `RequireSuperAdmin()` — that's the structural protection against
   forgetting the gate.
2. Secrets pass through `EffectiveValue.Mask()` before serialization,
   in exactly one code path (`toResponse`). No raw `EffectiveValue` is
   ever written to a response.
3. The audit log records every set/clear with the operator user_id but
   never the value — even for non-secrets. The audit feed is safe to
   share with reviewers without redaction.
4. Bootstrap-critical env vars (`JWT_SECRET`, `MFA_ENCRYPTION_KEY`,
   `DATABASE_URL`, `ENVIRONMENT`, `FRONTEND_URL`) are **excluded** from
   the catalog by design. Promoting any of them would create a
   chicken-and-egg: settings can't be decrypted until the key the
   settings store holds is itself decrypted.

If you spot a path that could leak a secret through this surface,
escalate per `SECURITY.md`.
