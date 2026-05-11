# Paper LMS

[![CI](https://github.com/EduThemes/paper-lms/actions/workflows/ci.yml/badge.svg)](https://github.com/EduThemes/paper-lms/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/EduThemes/paper-lms.svg)](https://pkg.go.dev/github.com/EduThemes/paper-lms)

**A Canvas-compatible learning management system for K-12 schools — single Go binary, modern React UI, no per-seat fees.**

Paper LMS speaks the Canvas REST API and ingests Canvas/IMSCC content packages, so districts can migrate without breaking their LTI tools, SIS integrations, or teacher workflows. Designed for K-12 from the ground up: K-2 picture-cue mode, parent observer accounts, FERPA/COPPA-aware models, OneRoster sync, and offline-friendly PWA.

> **Status:** Initial public release. 84 domain models, 360 API routes, 67 frontend pages.

---

## Why Paper LMS

| | Paper LMS | Canvas LMS | Google Classroom |
|---|---|---|---|
| Self-host (single binary) | ✅ | ❌ (Rails monorepo) | ❌ (SaaS only) |
| Canvas API compatibility | ✅ | ✅ | ❌ |
| K-12 specific (K-2 mode, parent accounts) | ✅ | Partial | Partial |
| LTI 1.3 / IMSCC import | ✅ | ✅ | ❌ |
| OneRoster v1.2 SIS sync | ✅ | ✅ | ❌ |
| SAML / LDAP / CAS SSO | ✅ | ✅ | Google-only |
| Per-seat licensing | Free | Paid | Free |
| Modern stack (Go + React + Tailwind) | ✅ | Rails + Ember | Closed |
| WCAG 2.1 AA | ✅ | ✅ | ✅ |
| Reading prefs (dyslexia, font size, TTS) | ✅ | ❌ | Limited |

**Migrate from Canvas in an afternoon:** point your Canvas API clients at Paper LMS, run `POST /api/v1/courses/:id/content_migrations` with your `.imscc` export, and you're live.

---

## Quickstart

```bash
git clone https://github.com/EduThemes/paper-lms.git
cd paper-lms
cp .env.example .env   # edit JWT_SECRET, DATABASE_URL, etc.
docker compose -f deployments/docker/docker-compose.prod.yml up -d
```

Open <http://localhost:8080>. The setup wizard creates your admin account on first run.

### Local development

```bash
# Backend (hot reload on save)
make build && ./paper-lms

# Frontend (Vite HMR on :5174)
cd web && npm install --legacy-peer-deps && npm run dev

# Apply database migrations
make migrate-up
```

Requires Go 1.25, Node 20+, PostgreSQL 14+ (with the optional `pgvector` extension for Smart Search).

---

## Features

### Core LMS
- **Courses, modules, assignments, quizzes, discussions, pages, files** — full Canvas-equivalent CRUD with the same workflow states (`active`, `unpublished`, `deleted`).
- **Gradebook** with virtualized scrolling, custom columns, late policies, mastery paths, conditional release, learning outcomes, and bulk operations (set default / curve / message students).
- **SpeedGrader** for one-by-one submission grading with rubrics, audio/video feedback, and originality reports.
- **Rich Content Editor** (TipTap-based) with KaTeX math, embedded media, autosave drafts, and AI assist (outline / summarize / rewrite via Anthropic Claude).

### K-12 differentiators
- **Course UI modes**: K-2 picture-cue layout with read-aloud, 3-5 simplified, 6-12 standard.
- **Parent / observer accounts** with pairing codes, multi-child switcher, weekly digest emails.
- **Reading preferences**: dyslexia-friendly fonts (Lexend, OpenDyslexic), spacing controls, italic-stripping, TTS toggle.
- **Mobile-first PWA** with offline support and a 5-item bottom nav optimized for thumb reach.

### Enterprise & integrations
- **Auth**: JWT cookies, OAuth 2.0, Personal Access Tokens, SAML 2.0, LDAP, CAS 2.0.
- **SIS**: OneRoster v1.2 CSV sync, Canvas SIS Imports CSV format, ad-hoc enrollment APIs.
- **LTI 1.3** + LTI Advantage (Names & Roles, Deep Linking, Assignment & Grade Services).
- **IMSCC** import/export (Common Cartridge 1.3) for content migration in/out of Canvas, Schoology, Moodle.
- **Storage**: pluggable backends — local disk, S3, MinIO, Cloudflare R2.
- **Email**: SMTP with weekly digests; per-user notification preferences.

### Additional capabilities
- **Discussion checkpoints** — multi-deadline thread participation ("post by Tue, reply twice by Fri").
- **Smart Search** — pgvector-backed semantic search across announcements, assignments, pages, and discussions.
- **Commons content library** — district-wide template sharing with favorites and one-click import.
- **AI Assist** in the rich content editor — Claude Haiku 4.5 powers Outline / Summarize / Rewrite, gated by per-user rate limit.
- **RTL support** — `tailwindcss-logical` plugin enabled across the component tree using logical properties.

---

## Architecture

```
cmd/server/main.go            # Composition root
internal/
  domain/models/              # 84 GORM models, Canvas-compatible
  repository/postgres/        # 81 repository implementations
  service/                    # 52 business-logic services
  api/v1/                     # Fiber HTTP handlers + middleware (60 handlers, 360 routes)
  auth/                       # JWT, OAuth2, SAML, LDAP, CAS
  graphql/                    # Hand-rolled GraphQL engine
  storage/                    # Pluggable file storage (local / S3)
web/src/
  pages/                      # 67 React pages (40 lazy-loaded chunks)
  components/                 # 27 shared components
  contexts/                   # Auth, theme, reading preferences
deployments/docker/           # Production Dockerfiles + nginx + compose
```

**Stack:** Go 1.25 · Fiber v2 · GORM v1.25 · PostgreSQL 14+ (pgvector optional) · React 18 · React Router 7 · Tailwind CSS 3.4 · Vite · TipTap.

**Bundle:** 417 KB main / 117 KB gzipped, with lazy chunks for TipTap (431 KB), Radix (138 KB), React (178 KB), KaTeX (lazy on math pages).

For deeper architectural detail and a "how to add a feature" cookbook, see [PROJECT.md](./PROJECT.md).

---

## Configuration

The full env var list lives in [`.env.example`](./.env.example). A few highlights:

| Variable | Default | Notes |
|---|---|---|
| `JWT_SECRET` | _(required in prod)_ | Auto-generated in dev; must be set for production. |
| `DATABASE_URL` | `postgres://localhost/paper_lms` | Standard libpq DSN. |
| `AUTO_MIGRATE` | `true` | GORM AutoMigrate (dev). Set `false` in prod and use `make migrate-up`. |
| `STORAGE_BACKEND` | `local` | `local` or `s3`. |
| `S3_BUCKET`, `S3_ENDPOINT` | | For S3, MinIO, or R2. |
| `ANTHROPIC_API_KEY` | | Enables AI Assist features. Without it, AI returns 503 cleanly. |
| `FRONTEND_URL` | | Required in production for CORS. |
| `SMTP_HOST` | | SMTP for digests and notifications. |

---

## Deployment

Production deployment is a single Go binary plus a built React bundle, fronted by nginx. The provided `docker-compose.prod.yml` wires up Postgres, the backend, and an nginx reverse proxy with sane TLS-ready defaults.

For Kubernetes, ECS, or systemd deployment notes, see [`docs/deployment/`](./docs/deployment/).

---

## Contributing

Issues and PRs welcome. Before opening a PR:

1. `go test ./...` and `cd web && npm run build` must pass.
2. Run `make vet` for static analysis.
3. New repositories follow the local-interface pattern (see [PROJECT.md](./PROJECT.md) — *Recipe: New API Endpoint*).
4. New routes use Canvas-style error format `{"errors":[{"message":"..."}]}` and Link-header pagination (RFC 5988).

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full workflow.

---

## Security

If you find a security issue, please email **michael.kocher@eduthemes.org** rather than opening a public issue. We aim to respond within 48 hours. See [SECURITY.md](./SECURITY.md) for the full disclosure policy and hardening guidance for self-hosted deployments.

---

## License & provenance

Paper LMS is released under the [MIT License](./LICENSE) — free to use, modify, distribute, and sublicense, with no warranty.

It is an **independent reimplementation** of the Canvas REST API and IMS Global specs. No source code from Canvas LMS, Schoology, Moodle, or any GPL/AGPL/SSPL project was used as a basis. Bundled dependencies are MIT, Apache-2.0, BSD, ISC, or MPL-2.0 file-scope only.

API contracts (field names, enum values, route shapes) are functional interfaces — per *Lotus v. Borland* and *Google v. Oracle America*, those aren't copyrightable as such.

"Canvas" is a trademark of Instructure, Inc. Paper LMS is not affiliated with, endorsed by, or sponsored by Instructure.

See [LICENSING.md](./LICENSING.md) for the full provenance statement and third-party software notes.
