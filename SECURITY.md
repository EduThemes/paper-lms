# Security Policy

Paper LMS handles student data — including for minors — and we take security seriously.

## Reporting a vulnerability

Please **do not open a public issue** for security reports. Email:

**michael.kocher@eduthemes.org**

Include:

- A description of the vulnerability and its impact
- Steps to reproduce (or a proof-of-concept)
- The affected version (commit SHA or release tag)
- Whether the issue has been disclosed elsewhere

I aim to acknowledge security reports within **48 hours** and provide a remediation timeline within **7 days**. Critical issues will be patched as quickly as possible; lower-severity issues are batched into the next release.

## Scope

In scope:

- The Paper LMS Go backend (under `cmd/`, `internal/`)
- The React frontend (under `web/`)
- The Docker deployment configuration (under `deployments/docker/`)
- Database migrations and authentication flows
- API endpoints under `/api/v1/`

Out of scope:

- Vulnerabilities in upstream dependencies (please report those upstream — we'll bump versions when fixes are released)
- Self-XSS or attacks requiring physical device access
- Social engineering of project maintainers
- Denial-of-service attacks against your own deployment (rate-limit configuration is your responsibility)

## Disclosure policy

We follow **coordinated disclosure**:

1. You report privately.
2. We confirm and develop a fix.
3. We release the patch and credit you (if you'd like) in the release notes.
4. We disclose details publicly **30 days after the patched release** so deployments have time to upgrade.

If we cannot reach agreement on a timeline, you are welcome to disclose at your discretion — please give us reasonable notice.

## Hardening guidance for self-hosted deployments

Recommended for any production install:

- Set `JWT_SECRET` to at least 32 random bytes (`openssl rand -hex 32`).
- Set `ENVIRONMENT=production` so config validation runs.
- Set `AUTO_MIGRATE=false` and run versioned migrations explicitly.
- Front the Go server with TLS (the supplied nginx config is a good starting point).
- Use a dedicated Postgres user with the minimum required privileges.
- Rotate `JWT_SECRET` if you suspect compromise (this invalidates all sessions).
- Configure a Content Security Policy via the existing CSP middleware; the default is enforce mode.
- Keep `.env` out of source control — `.gitignore` already excludes it.

## Known intentional behaviors (not vulnerabilities)

- The backend exposes detailed error messages by default. For production, set the appropriate logging level and consider an upstream WAF.
- The default development `JWT_SECRET` triggers a fatal error in production mode. This is intentional.
- AI Assist endpoints return 503 when `ANTHROPIC_API_KEY` is unset. This is intentional graceful degradation, not a vulnerability.

## Recognition

Researchers who report valid security issues responsibly will be credited (with permission) in the release notes and a future `SECURITY-HALL-OF-FAME.md`. Thanks for helping keep students and teachers safe.
