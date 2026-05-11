# Paper LMS — Project Guide

A developer-facing reference for the codebase. Pairs with the user-facing
[README](./README.md): the README sells the project; this file is how to
work inside it.

## Project Overview
Paper LMS is a production-ready, Canvas LMS-backwards-compatible learning management system for K-12 schools. Built with Go (backend) and React (frontend), targeting exact Canvas API compatibility so teachers can migrate from Canvas without losing content, LTI tools, or SIS integrations.

## Tech Stack
- **Backend**: Go 1.24 + Fiber v2.52.6 + GORM v1.25.10 + PostgreSQL
- **Frontend**: React 18 + React Router 7 + Tailwind CSS 3.4.1 + Vite
- **Module path**: `github.com/EduThemes/paper-lms`

## Project Structure
```
paper-LMS/
  cmd/
    server/main.go                    # Composition root (wires repos → services → handlers → router)
    migrate/main.go                   # Database migration CLI tool
    genschema/main.go                 # Schema SQL generator (dev tool)
  internal/
    config/config.go                  # Centralized env config
    domain/models/                    # Canvas-compatible model structs (84 files)
    repository/
      interfaces.go                   # All repository interfaces
      postgres/                       # GORM implementations (81 files)
    service/                          # Business logic layer (52 files)
    auth/                             # SSO: SAML 2.0, LDAP, CAS 2.0
    graphql/                          # Hand-rolled GraphQL engine
    api/v1/
      router.go                       # Route registration (360 routes)
      middleware/                     # Auth, pagination, RBAC, rate limiting, security headers
      handlers/                       # HTTP handlers (60 files)
      responses/                      # Pagination, error format helpers
    db/
      postgres.go                     # PostgreSQL connection + AutoMigrate
      migrate.go                      # golang-migrate runner (embedded SQL)
      migrations/                     # Versioned SQL migration files
    storage/                          # Pluggable file storage (local disk, S3/MinIO/R2)
    testutil/                         # Test mocks & utilities
  web/src/
    pages/                            # React pages (67 files)
    components/                       # Layout, ProtectedRoute, RichContentEditor, CourseNav, etc. (27 files)
    services/api.js                   # API client with Canvas Link-header pagination
    hooks/                            # useIsTeacher, useUnsavedChanges, useCourseVisitTracker
    contexts/                         # AuthContext (JWT), CourseUIContext (K-2/3-5 mode)
    utils/                            # Shared utilities (grading.js, etc.)
  deployments/docker/                 # Dockerfiles, nginx.conf, docker-compose.prod.yml
  .github/workflows/ci.yml           # GitHub Actions (lint, test, build, docker)
```

## Build Commands
```bash
# Backend
go build ./...                        # or: make build
go vet ./...                          # or: make vet
go test ./...                         # or: make test

# Frontend
cd web && npm run build               # or: make frontend-build
cd web && npm run dev                  # or: make frontend-dev

# Database migrations
make migrate-up                       # Apply all pending migrations
make migrate-down                     # Roll back last migration
make migrate-create                   # Create new migration files

# Docker
docker compose -f deployments/docker/docker-compose.prod.yml up
```

## Key Patterns

### Architecture (Clean Architecture)
- **Repository pattern**: Interfaces in `interfaces.go`, GORM implementations in `postgres/`
- **Service layer**: Business logic with dependency injection of repository interfaces
- **Handler layer**: Fiber HTTP handlers that parse requests, call services, format responses
- **Wiring**: `cmd/server/main.go` wires repos → services → handlers → router → Fiber app

### Canvas API Compatibility
- All endpoints under `/api/v1/`
- Error format: `{"errors": [{"message": "..."}]}`
- Pagination: Link headers (RFC 5988) via `responses.SetPaginationHeaders`
- Soft deletes via `workflow_state` field (set to "deleted", never hard delete)

### Auth & RBAC
- JWT (HS256) httpOnly cookie `paper_session` + OAuth2 + Personal Access Tokens + SAML/LDAP/CAS SSO
- RBAC middleware: `RequireAdmin`, `RequireInstructor`, `RequireEnrolled`, `RequireSelfOrAdmin`
- Frontend: `useAuth()` context, `useIsTeacher(courseId)` hook for role detection

### Frontend Conventions
- Role detection: `useIsTeacher(courseId)` returns `null` (loading) / `true` / `false`
- API responses: always use `result.data || []` fallback for null safety
- Code splitting: `React.lazy()` for non-hot-path pages, static imports for Dashboard/Course/Assignments
- Icons: Lucide React (import individually, e.g., `import { Eye } from 'lucide-react'`)
- Loading states: animated SVG spinner (never plain "Loading..." text)
- Error states: always include "Try Again" button

## Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `AUTO_MIGRATE` | `true` | GORM AutoMigrate (dev). Set `false` for SQL migrations (prod). |
| `STORAGE_BACKEND` | `local` | File storage: `local` or `s3` |
| `S3_BUCKET` | | S3 bucket name |
| `S3_ENDPOINT` | | Custom endpoint for MinIO/R2/GCS |
| `JWT_SECRET` | | Required in production (auto-generates in dev) |
| `FRONTEND_URL` | | Required in production for CORS |
| `SMTP_HOST` | | SMTP server for email notifications |

---

## Cookbook: Adding a New Feature

### Recipe: New API Endpoint (full stack)

**Step 1: Model** — `internal/domain/models/thing.go`
```go
package models

import "time"

type Thing struct {
    ID            uint      `json:"id" gorm:"primaryKey"`
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
    FindByID(ctx context.Context, id uint) (*models.Thing, error)
    Update(ctx context.Context, thing *models.Thing) error
    Delete(ctx context.Context, id uint) error
    ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.Thing], error)
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

func (r *thingRepo) FindByID(ctx context.Context, id uint) (*models.Thing, error) {
    var thing models.Thing
    if err := r.db.WithContext(ctx).First(&thing, id).Error; err != nil {
        return nil, err
    }
    return &thing, nil
}

func (r *thingRepo) Update(ctx context.Context, thing *models.Thing) error {
    return r.db.WithContext(ctx).Save(thing).Error
}

func (r *thingRepo) Delete(ctx context.Context, id uint) error {
    return r.db.WithContext(ctx).Model(&models.Thing{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *thingRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Thing], error) {
    var items []models.Thing
    var totalCount int64
    query := r.db.WithContext(ctx).Model(&models.Thing{}).Where("course_id = ? AND workflow_state != 'deleted'", courseID)
    query.Count(&totalCount)
    offset := (params.Page - 1) * params.PerPage
    if err := query.Order("created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
        return nil, err
    }
    return &repository.PaginatedResult[models.Thing]{Items: items, TotalCount: totalCount, Page: params.Page, PerPage: params.PerPage}, nil
}
```

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
    params := middleware.GetPagination(c)
    result, err := h.thingRepo.ListByCourseID(c.Context(), uint(courseID), params)
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
4. `internal/db/migrations/` — Add SQL migration for production

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
5. If `AUTO_MIGRATE=true`, GORM handles the column automatically
6. For production: add SQL migration in `internal/db/migrations/`

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
- **84 models**, **81 repos**, **52 services**, **60 handlers**, **360 API routes**
- **67 frontend pages**, **27 shared components**, **3 hooks**, **2 contexts**
- **40 lazy-loaded chunks** — main bundle 267KB / 56KB gzipped
- Auth: JWT + OAuth2 + PAT + SAML/LDAP/CAS SSO
- Storage: pluggable local disk / S3
- CI/CD: GitHub Actions, Docker, health probes
- PWA with service worker + offline support
- WCAG 2.1 AA accessibility

## Known Issues / Technical Debt
- Frontend test coverage is minimal (5 test files)
- No drag-and-drop reorder for modules/module items
- FERPA cascade delete not implemented
- Email: no dead letter queue, bounce handling, or delivery confirmation
- Single-tenant architecture (no multi-tenancy)
- Backend requires restart for Go changes (Vite HMR works for frontend)

## Future Features

### Gamification engine (planned)
A trigger-driven gamification layer modeled after WordPress plugins like GamiPress and myCred. Goals:
- **Points / XP**: multiple named point types per account (e.g., XP, Coins, Reputation), per-user balances, full transaction ledger.
- **Badges / achievements**: definitions with icon, title, criteria; awards table tracks who earned what and when.
- **Leaderboards**: course-scoped, account-scoped, and global; configurable point type and time window (all-time, term, week).
- **Triggers**: declarative rules wired to existing domain events — assignment submitted, quiz passed (≥ score), discussion replied, module completed, attendance streak, peer review submitted, mastery target hit. Each trigger can grant points, award a badge, or fire a webhook.
- **Manual awards**: instructor/admin UI to grant points or badges, with reason and audit log.
- **Rules engine**: composable conditions (AND/OR), cooldowns, and per-user/per-course caps to prevent farming.
- **Notifications**: tie into existing notification system so award events surface in the bell + email/SMS prefs.
- **Student-facing**: profile widget showing balance, recent awards, and progress toward next badge; opt-out per user (FERPA-conscious — leaderboard display name controls).
- **API parity**: Canvas doesn't have direct equivalents, so design fresh `/api/v1/gamification/*` endpoints with full pagination + Link headers.

Implementation sketch (when picked up): new domain models (`PointType`, `PointTransaction`, `Badge`, `BadgeAward`, `Trigger`, `TriggerRule`, `LeaderboardSnapshot`), a trigger dispatcher hooked into the service layer (publish events from Submission, Quiz, Discussion, Module, Attendance services), and a React Gamification page set under both teacher and student nav.
