# Paper LMS — Project Guide

A developer-facing reference for the codebase. Pairs with the user-facing
[README](./README.md): the README sells the project; this file is how to
work inside it.

## Project Overview
Paper LMS is a production-ready, Canvas LMS-backwards-compatible learning
management system for K-12 schools. Built with Go (backend) and React
(frontend), targeting exact Canvas API compatibility so teachers can
migrate from Canvas without losing content, LTI tools, or SIS integrations.

The codebase is multi-tenant (per-account isolation enforced at the
repository layer — see [ADR 0001](./docs/adr/0001-canvas-compatible-api-shape.md)
for API shape and Phase 13 in `CHANGELOG.md` for tenant scoping).

## Tech Stack
- **Backend**: Go 1.25 + Fiber v2.52 + GORM v1.25 + PostgreSQL 14+ (with
  optional `pgvector` extension for Smart Search; backend dev image is
  `pgvector/pgvector:pg16`)
- **Frontend**: React 18 + React Router 7 + Tailwind CSS 3.4 + Vite
- **Module path**: `github.com/EduThemes/paper-lms`

## Project Structure
```
paper-LMS/
  cmd/
    server/main.go                    # Composition root (wires repos → services → handlers → router)
    migrate/main.go                   # Database migration CLI tool
    genschema/main.go                 # Schema SQL generator (dev tool)
    schemadiff/                       # GORM-vs-SQL parity tool (`make schema-diff`)
    stalecols/                        # SQL-chain stale-column reporter (`make stale-cols`)
    leaderboard-snapshot/             # Weekly gamification leaderboard snapshot CLI
  internal/
    config/config.go                  # Centralized env config
    domain/models/                    # Canvas-compatible model structs
    repository/
      interfaces.go                   # All repository interfaces
      postgres/                       # GORM implementations
    service/                          # Business logic layer
    auth/                             # SSO + login pipeline + secretbox (AES-256-GCM)
    settingsctx/                      # Per-tenant settings resolution
    graphql/                          # Hand-rolled GraphQL engine
    api/v1/
      router.go                       # Route registration
      middleware/                     # Auth, pagination, RBAC, rate limiting, security headers, audit
      handlers/                       # HTTP handlers
      responses/                      # Pagination, error format helpers
    db/
      postgres.go                     # PostgreSQL connection + AutoMigrate
      migrate.go                      # golang-migrate runner (embedded SQL)
      migrations/                     # Versioned SQL migration files (000001..000060)
    storage/                          # Pluggable file storage (local disk, S3/MinIO/R2)
    testutil/                         # Test mocks & utilities
  web/src/
    pages/                            # React pages
    components/                       # Layout, ProtectedRoute, RichContentEditor, CourseNav, etc.
    services/api.js                   # API client with Canvas Link-header pagination
    hooks/                            # useIsTeacher, useUnsavedChanges, useCourseVisitTracker
    contexts/                         # AuthContext (JWT), CourseUIContext (K-2/3-5 mode)
    utils/                            # Shared utilities (grading.js, etc.)
  docs/
    adr/                              # Architecture Decision Records (start here for "why")
    audits/                           # Security + correctness audits (see SECURITY.md)
    auth/                             # OIDC provider walkthroughs
    state-dpa/                        # State DPA / multi-pod verification runbooks
    status/                           # Per-phase handoff notes (full state-of-the-world)
  deployments/docker/                 # Dockerfiles, nginx.conf, docker-compose.prod.yml
  .github/workflows/ci.yml            # GitHub Actions (lint, test, build, axe, docker)
```

## Build Commands
```bash
# Show every Makefile target with one-line descriptions
make help                             # also the default target

# Backend
go build ./...                        # or: make build
go vet ./...                          # or: make vet
go test ./...                         # or: make test

# Frontend
cd web && npm run build               # or: make frontend-build
cd web && npm run dev                 # or: make frontend-dev

# Database migrations (production: AUTO_MIGRATE=false)
make migrate-up                       # Apply all pending migrations
make migrate-down                     # Roll back last migration
make migrate-create                   # Scaffold a new versioned migration pair

# Schema parity (run before merging a model change)
make schema-diff                      # GORM AutoMigrate vs SQL chain
make stale-cols                       # SQL chain has columns AutoMigrate doesn't

# Docker
docker compose -f deployments/docker/docker-compose.prod.yml up
```

See `make help` for the rest (`dex-up`, `dex-down`, `dex-logs`, `backup`,
`restore`, etc.). Architectural "why" decisions are in
[`docs/adr/`](./docs/adr/README.md).

## Key Patterns

### Architecture (Clean Architecture)
- **Repository pattern**: Interfaces in `interfaces.go`, GORM implementations in `postgres/`
- **Service layer**: Business logic with dependency injection of repository interfaces
- **Handler layer**: Fiber HTTP handlers that parse requests, call services, format responses
- **Wiring**: `cmd/server/main.go` wires repos → services → handlers → router → Fiber app

### Canvas API Compatibility ([ADR 0001](./docs/adr/0001-canvas-compatible-api-shape.md))
- All endpoints under `/api/v1/`
- Error format: `{"errors": [{"message": "..."}]}`
- Pagination: Link headers (RFC 5988) via `responses.SetPaginationHeaders`
- Soft deletes via `workflow_state` field (set to "deleted", never hard delete)

### Multi-tenancy
- Every business row carries `account_id`; handlers MUST pass
  `callerAccountID(c)` into repos. `accountID == 0` is reserved for
  internal background callers only (see Phase 13 patterns in
  `CHANGELOG.md` and Phase 13 status notes).
- Cross-tenant access returns 404, not 403, to avoid existence leaks
  (`responses.NotFound` + `assertSameTenant`).
- Verification harness: `internal/api/v1/handlers/tenant_isolation_test.go`
  (28-case 2×2 matrix, runs in CI).

### Migrations + Schema Parity ([ADR 0002](./docs/adr/0002-auto-migrate-policy.md))
- **`AUTO_MIGRATE=false` in prod / CI; `AUTO_MIGRATE=true` in dev.**
- The SQL chain in `internal/db/migrations/` is the source of truth.
  Adding a model requires the matching migration in the same PR.
- `TestSchemaParity_Wave1` / `Wave3` are CI hard-fails on stale columns,
  missing columns, missing indexes.

### Auth & RBAC ([ADR 0004](./docs/adr/0004-login-pipeline-mfa-gate-placement.md))
- JWT (HS256) httpOnly cookie `paper_session` + OAuth2 + Personal Access
  Tokens + SAML/LDAP/CAS SSO + OIDC + WebAuthn passkeys.
- Every credential type funnels through `auth.LoginPipeline.Execute`;
  the handler emits an `SSOOutcome` and the pipeline decides the MFA
  gate, session minting, and audit log.
- RBAC middleware: `RequireAdmin`, `RequireInstructor`,
  `RequireEnrolled`, `RequireSelfOrAdmin`.
- Frontend: `useAuth()` context, `useIsTeacher(courseId)` hook for role
  detection.

### Encryption at Rest ([ADR 0003](./docs/adr/0003-secretbox-encryption-at-rest.md))
- `internal/auth/secretbox.go` — AES-256-GCM envelope, versioned
  `key_id` byte for rotation. Key from `MFA_ENCRYPTION_KEY` (32 bytes
  base64).
- Any DB-resident secret (TOTP secret, OIDC client_secret, LDAP bind
  password) MUST round-trip through `secretbox.Encrypt`.

### Leaderboard render policy ([ADR 0005](./docs/adr/0005-render-policy-single-source-of-truth.md))
- `gamification.RenderPolicyFor(tenantMode, role, viewerRank)` is the
  only place leaderboard visibility, pseudonyms, and top-N gating are
  decided. K-5 students never see top-N; HigherEd / Corp / Pro see real
  names; admin + teacher always see truth.

### Frontend Conventions
- Role detection: `useIsTeacher(courseId)` returns `null` (loading) / `true` / `false`
- API responses: always use `result.data || []` fallback for null safety
- Code splitting: `React.lazy()` for non-hot-path pages, static imports for Dashboard/Course/Assignments
- Icons: Lucide React (import individually, e.g., `import { Eye } from 'lucide-react'`)
- Loading states: animated SVG spinner (never plain "Loading..." text)
- Error states: always include "Try Again" button
- Brand mark: single `<BrandLogo />` (`web/src/components/brand/BrandLogo.jsx`)
  fed by `web/public/brand/paper-logo.svg` — swap one file to rebrand.

## Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `AUTO_MIGRATE` | `true` | GORM AutoMigrate (dev). Set `false` for SQL migrations (prod/CI). |
| `STORAGE_BACKEND` | `local` | File storage: `local` or `s3` |
| `S3_BUCKET` | | S3 bucket name |
| `S3_ENDPOINT` | | Custom endpoint for MinIO/R2/GCS |
| `JWT_SECRET` | | Required in production (auto-generates in dev) |
| `MFA_ENCRYPTION_KEY` | | 32-byte base64; required when any encrypted secret column is in use (TOTP, OIDC, LDAP) |
| `FRONTEND_URL` | | Required in production for CORS |
| `SMTP_HOST` | | SMTP server for email notifications |
| `PASSKEY_RPID` | `localhost` | WebAuthn relying-party ID |
| `PASSKEY_RPORIGINS` | `${FRONTEND_URL}` | Comma-separated allowed origins for passkey ceremonies |
| `REDIS_URL` | | Optional; switches rate-limit store from in-memory to Redis |

The full env list with descriptions is in [`.env.example`](./.env.example).

---

## Phase status

The phase / wave nomenclature comes from the commit history. Treat this
as a sketch; the authoritative state is in [`docs/status/`](./docs/status/)
and the unreleased section of [`CHANGELOG.md`](./CHANGELOG.md).

- **Phase 6 (gamification)** — engine, rules, badges, currencies,
  leaderboards, pseudonym layer, relative window + filler users,
  weekly snapshots. Currencies / badges / recipes editable in-app at
  `/admin/gamification/*`. Audit at
  [`docs/audits/2026-05-15-gamification-audit.md`](./docs/audits/2026-05-15-gamification-audit.md).
- **Phase 9 / 10 (auth)** — `LoginPipeline` convergence; SAML / LDAP /
  CAS / OIDC / WebAuthn passkeys / TOTP + recovery codes; per-tenant
  MFA policy; secretbox-encrypted secret columns; brute-force tracker.
- **Phase 12 (university launch hardening)** — panic recovery, HTTP
  timeouts, SIGTERM hoist, observer pairing-code IDOR fix, request body
  limit, deep `/ready` checks. Audit grade C- → B-.
- **Phase 13 (multi-tenancy)** — per-tenant `account_id` filtering
  across ~35 repositories + handlers + 3 high-leverage services;
  COPPA gates; FERPA cascade-delete service; append-only audit-log
  trigger; i18n extraction + es.json translation; axe CI gate. Two
  documented LEAK contracts surfaced by `tenant_isolation_test.go`.
- **Settings engine** — per-tenant catalog at
  `internal/service/settings/catalog.go`; isolation tested at
  `internal/api/v1/handlers/super_admin_isolation_test.go`.

## Cookbook: Adding a New Feature

### Recipe: New API Endpoint (full stack)

**Step 1: Model** — `internal/domain/models/thing.go`
```go
package models

import "time"

type Thing struct {
    ID            uint      `json:"id" gorm:"primaryKey"`
    AccountID     uint      `json:"account_id" gorm:"not null;index"`
    CourseID      uint      `json:"course_id" gorm:"not null;index"`
    Title         string    `json:"title" gorm:"not null"`
    Description   string    `json:"description" gorm:"type:text"`
    WorkflowState string    `json:"workflow_state" gorm:"not null;default:'active'"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

**Step 2: Repository interface** — Add to `internal/repository/interfaces.go`
```go
type ThingRepository interface {
    Create(ctx context.Context, thing *models.Thing) error
    FindByID(ctx context.Context, id uint, accountID uint) (*models.Thing, error)
    Update(ctx context.Context, thing *models.Thing) error
    Delete(ctx context.Context, id uint, accountID uint) error
    ListByCourseID(ctx context.Context, courseID uint, accountID uint, params PaginationParams) (*PaginatedResult[models.Thing], error)
}
```

**Step 3: Repository implementation** — `internal/repository/postgres/thing_repo.go`
```go
package postgres

import (
    "context"
    "github.com/EduThemes/paper-lms/internal/domain/models"
    "github.com/EduThemes/paper-lms/internal/repository"
    "gorm.io/gorm"
)

type thingRepo struct{ db *gorm.DB }

func NewThingRepository(db *gorm.DB) *thingRepo {
    return &thingRepo{db: db}
}

func (r *thingRepo) Create(ctx context.Context, thing *models.Thing) error {
    return r.db.WithContext(ctx).Create(thing).Error
}

func (r *thingRepo) FindByID(ctx context.Context, id uint, accountID uint) (*models.Thing, error) {
    var thing models.Thing
    q := r.db.WithContext(ctx).Where("id = ?", id)
    if accountID != 0 {
        q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
    }
    if err := q.First(&thing).Error; err != nil {
        return nil, err
    }
    return &thing, nil
}
```

(see `internal/repository/postgres/course_repo.go` for the canonical
tenant-scoped repo shape, and the Phase 13 pattern in `CHANGELOG.md`)

**Step 4: Service** (if business logic needed) — `internal/service/thing_service.go`
```go
package service

import (
    "context"
    "errors"
    "github.com/EduThemes/paper-lms/internal/domain/models"
    "github.com/EduThemes/paper-lms/internal/repository"
)

type ThingService struct {
    thingRepo repository.ThingRepository
}

func NewThingService(thingRepo repository.ThingRepository) *ThingService {
    return &ThingService{thingRepo: thingRepo}
}

func (s *ThingService) GetByID(ctx context.Context, id uint, accountID uint) (*models.Thing, error) {
    return s.thingRepo.FindByID(ctx, id, accountID)
}

func (s *ThingService) Create(ctx context.Context, thing *models.Thing) error {
    if thing.Title == "" {
        return errors.New("title is required")
    }
    return s.thingRepo.Create(ctx, thing)
}
```

**Step 5: Handler** — `internal/api/v1/handlers/things.go`
```go
package handlers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/EduThemes/paper-lms/internal/api/v1/middleware"
    "github.com/EduThemes/paper-lms/internal/api/v1/responses"
    "github.com/EduThemes/paper-lms/internal/domain/models"
    "github.com/EduThemes/paper-lms/internal/repository"
)

type ThingHandler struct {
    thingRepo repository.ThingRepository
}

func NewThingHandler(thingRepo repository.ThingRepository) *ThingHandler {
    return &ThingHandler{thingRepo: thingRepo}
}

func thingToJSON(t *models.Thing) fiber.Map {
    return fiber.Map{
        "id": t.ID, "course_id": t.CourseID, "title": t.Title,
        "description": t.Description, "workflow_state": t.WorkflowState,
        "created_at": t.CreatedAt, "updated_at": t.UpdatedAt,
    }
}

func (h *ThingHandler) List(c *fiber.Ctx) error {
    courseID, err := c.ParamsInt("course_id")
    if err != nil { return responses.BadRequest(c, "Invalid course ID") }
    accountID := callerAccountID(c)
    params := middleware.GetPagination(c)
    result, err := h.thingRepo.ListByCourseID(c.Context(), uint(courseID), accountID, params)
    if err != nil { return responses.InternalError(c, "Could not fetch things") }
    responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
    items := make([]fiber.Map, len(result.Items))
    for i, t := range result.Items { items[i] = thingToJSON(&t) }
    return c.JSON(items)
}
```

**Step 6: Wire it up** — Modify these shared files (main thread only, never in parallel agents):
1. `internal/db/postgres.go` — Add `&models.Thing{}` to AutoMigrate list
2. `internal/api/v1/router.go` — Add handler field, constructor param, route registration
3. `cmd/server/main.go` — Initialize repo, service, handler; pass to router
4. `internal/db/migrations/` — Add SQL migration for production (`make migrate-create`)

**Step 7: Frontend API method** — Add to `web/src/services/api.js`
```js
getThings: (courseId, page = 1, perPage = 10) =>
    request(`/courses/${courseId}/things?page=${page}&per_page=${perPage}`),
createThing: (courseId, data) =>
    request(`/courses/${courseId}/things`, { method: 'POST', body: JSON.stringify(data) }),
```

**Step 8: Frontend page** — `web/src/pages/ThingsPage.jsx`
```jsx
import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const ThingsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [things, setThings] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetch = async () => {
      try {
        const result = await api.getThings(courseId);
        setThings(result.data || []);
      } catch (err) { setError(err.message); }
      finally { setLoading(false); }
    };
    fetch();
  }, [courseId]);

  return (
    <Layout>
      <CourseNav courseId={courseId} />
      <div className="p-6">
        {/* loading spinner, error with retry, content */}
      </div>
    </Layout>
  );
};
export default ThingsPage;
```

**Step 9: Route** — Add to `web/src/App.jsx`
```jsx
const ThingsPage = React.lazy(() => import('./pages/ThingsPage'));
// In routes:
<Route path="/courses/:courseId/things" element={<Suspense><ThingsPage /></Suspense>} />
```

### Recipe: Adding a Field to an Existing Model
1. Add field to model struct in `internal/domain/models/xxx.go`
2. Add field to `xxxToJSON()` in the handler
3. Add field to create/update input struct in the handler
4. Add field to frontend API calls and page state
5. If `AUTO_MIGRATE=true`, GORM handles the column in dev
6. **Required for production**: add SQL migration via `make migrate-create`
7. Run `make schema-diff` to confirm parity before pushing

### Recipe: Adding a New React Page (no backend changes)
1. Create `web/src/pages/XxxPage.jsx` following the page pattern above
2. Add lazy import + route in `web/src/App.jsx`
3. Add to CourseNav tabs if course-scoped (in `web/src/components/CourseNav.jsx`)
4. Add nav link in `web/src/components/Layout.jsx` if app-level

## Shared Files (modify only from main thread)
When using parallel agents, these files must only be edited by the main thread to avoid conflicts:
- `internal/repository/interfaces.go`
- `internal/db/postgres.go` (AutoMigrate list)
- `internal/api/v1/router.go` (route registration)
- `cmd/server/main.go` (dependency wiring)
- `web/src/services/api.js` (API methods)
- `web/src/App.jsx` (React routes)
- `web/src/components/Layout.jsx` (nav links)

Agents should ONLY create new files. All shared file edits happen in the main thread.

## Current State

Counts are regenerated from the live tree (2026-05-17). Re-run the
commands documented at the top of this file (`find internal/... | wc -l`)
to refresh.

- **118 models**, **120 repos**, **133 services**, **99 handlers** (counts as of 2026-05-17)
- **96 frontend pages**, plus shared components, hooks, contexts
- **60 SQL migrations** in `internal/db/migrations/` (latest: `000060_backfill_ldap_bind_password_encrypted`)
- Auth: JWT + OAuth2 + PAT + SAML / LDAP / CAS / OIDC / WebAuthn passkeys + TOTP MFA
- Storage: pluggable local disk / S3 / MinIO / R2
- CI/CD: GitHub Actions (lint, test, build, axe, docker), rolling-restart auto-deploy on push to main
- PWA with service worker + offline support
- WCAG 2.1 AA accessibility (axe CI gate, critical-only)
- i18n: en.json + es.json, language switcher in Layout sidebar

## Known Issues / Technical Debt
- Frontend test coverage is minimal (handful of test files)
- No drag-and-drop reorder for modules / module items
- Email pipeline lacks a dead-letter queue and bounce handling
- Backend requires restart for Go changes (Vite HMR works for frontend)
- **Tenant-scope LEAK contracts** still surfaced by
  `tenant_isolation_test.go`: `SubmissionRepository.FindByAssignmentAndUser`
  signature needs widening; `Conversation.requireParticipant` returns
  403 instead of 404. Both are documented in CI output.
- **GraphQL resolver tenant scope** is partially plumbed — `context.go`
  carries `account_id`; remaining service calls need to thread it.
- The rate-limit store defaults to in-memory; multi-pod deployments
  must set `REDIS_URL`.

## Further Reading

- [`docs/adr/`](./docs/adr/README.md) — Architecture Decision Records
- [`docs/audits/`](./docs/audits/) — Security + correctness audits
- [`docs/status/`](./docs/status/) — Per-phase handoff notes
- [`docs/auth/oidc-providers.md`](./docs/auth/oidc-providers.md) — OIDC setup walkthroughs
- [`docs/state-dpa/`](./docs/state-dpa/) — State DPA dry-run + multi-pod runbooks
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — Workflow + the two-migration rename pattern
- [`SECURITY.md`](./SECURITY.md) — Threat model + disclosure policy
