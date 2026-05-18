# 0001. Canvas-compatible API shape

## Status

Accepted

## Context

Paper LMS targets K-12 districts that are already running Canvas LMS.
Districts have invested heavily in Canvas-shaped tooling: SIS imports,
LTI 1.3 launches, mobile clients, scripts hitting `/api/v1/...`,
gradebook integrations, parent-app pollers. A migration target that
asks them to rewrite all of that is dead on arrival.

Canvas's public REST API is an industry de facto standard. The route
shapes, the JSON error envelope, the `workflow_state` lifecycle, and
the RFC 5988 Link-header pagination are all functional interfaces —
per *Lotus v. Borland* and *Google v. Oracle America*, those are
not copyrightable as such. We can implement the same wire contract
without using any Canvas source.

## Decision

**Every public HTTP API mirrors Canvas LMS at the wire level:**

- All endpoints live under `/api/v1/`. The route layout follows
  Canvas's REST API documentation
  (`/api/v1/courses/:id/assignments`, `/api/v1/users/:id/enrollments`, etc.).
  Registration happens in `internal/api/v1/router.go`.
- Error envelope is **always** `{"errors": [{"message": "..."}]}`.
  Helpers in `internal/api/v1/responses/` enforce this format; new
  handlers MUST use them.
- Pagination uses Link headers (RFC 5988) via
  `responses.SetPaginationHeaders` — no offset/limit in the response
  body, no `next_page` / `prev_page` fields. The frontend's
  `web/src/services/api.js` reads the Link header.
- Lifecycle is `workflow_state` (`active`, `unpublished`, `deleted`,
  etc.). Soft delete sets the field to `"deleted"`. No hard deletes,
  no `deleted_at` timestamps, no GORM soft-delete column.
- Field names match Canvas: snake_case JSON, `created_at` / `updated_at`,
  `course_id` rather than `courseId`.

Where Paper LMS extends Canvas (gamification, parent-observer pairing
codes, K-2 picture-cue mode), we add fresh endpoints under a sibling
namespace (e.g. `/api/v1/gamification/*`) and keep the rest of the
surface unchanged.

## Consequences

- **Migration becomes a configuration change** for districts on Canvas:
  point existing clients at Paper LMS, run a Canvas IMSCC export through
  `POST /api/v1/courses/:id/content_migrations`, done.
- **Every new endpoint** must use the Canvas error envelope and
  Link-header pagination. PR review treats deviations as bugs.
- **No GORM `DeletedAt` columns** — soft delete is part of the
  business workflow, not an ORM concern. Repos filter
  `workflow_state != 'deleted'` explicitly.
- **API surface is large** (300+ routes) because Canvas's is. We
  resist the urge to "consolidate" routes that look redundant — many
  of them are load-bearing for specific Canvas clients.
- The README front-page comparison table and the `/api/v1/` namespace
  are the externally visible contract; ADR-level changes here are
  rare and require a major-version bump.

## References

- `internal/api/v1/router.go` — route registration
- `internal/api/v1/responses/` — error envelope + Link-header helpers
- `README.md` — Canvas-compatibility comparison table
- `LICENSING.md` — provenance statement (no Canvas source used)
