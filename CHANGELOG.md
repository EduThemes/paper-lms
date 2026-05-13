# Changelog

All notable changes to Paper LMS are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Phase 6 / Wave 2 Sprint W2-B follow-up — admin nav entry + plural-label clarity

Closes the discoverability gap on the currency editor: it was reachable
only by typing `/admin/gamification/currencies` directly. Also makes the
singular/plural distinction in the editor self-explanatory.

- `AdminNav` gains a new "Gamification" group inside the "More…"
  popover with a "Currencies" entry (Coins lucide icon) pointing at
  `/admin/gamification/currencies`. Placed at the end of the
  secondary-groups list because it's not yet a daily-frequent surface;
  the group will grow to "Recipes" / "Badges" / "Leaderboards" in
  W2-D/E and may promote to a top-level nav entry once that happens.
- `CurrencyEditor` renames "Display label" → "Display label (singular)"
  and adds one-line hints under both that field and "Plural label"
  showing exactly when each form is rendered (`"You earned 1 Coin"` vs
  `"You earned 4 Coins"`). The backend has always stored both forms;
  the W2-A `WalletDrawer` header already uses the plural for non-1
  balances. The W2-D notification surface will pick singular vs plural
  off `amount === 1` at render time.

### Phase 6 / Wave 2 Sprint W2-B review follow-up — race-safe duplicate detection

`/review` on PR #13 flagged the only real risk: the original
`CreateCurrency` ran `FindByCode` then `Create` non-atomically. Two
concurrent admins minting the same code could both pass the pre-check;
one would then surface the unique-constraint hit as a generic 500.

Closed by collapsing duplicate detection into a single atomic statement:

- `GamificationCurrencyTypeRepo.Create` now uses
  `INSERT … ON CONFLICT ON CONSTRAINT uniq_gam_currency_scope_code DO
  NOTHING RETURNING id, created_at, updated_at`. Conflict → zero rows →
  `sql.ErrNoRows` on the Scan → translated to a new typed sentinel
  `repository.ErrCurrencyDuplicate`.
- `CreateCurrency` handler drops the `FindByCode` pre-check entirely
  and switches its conflict path to `errors.Is(err,
  repository.ErrCurrencyDuplicate) → 409`. One DB round-trip per POST
  instead of two; race-free under any interleaving.
- `TestCreateCurrency_Duplicate_Returns409` rewritten to assert via
  the sentinel rather than a stubbed `FindByCode` hit. Other Create
  tests dropped their now-unused `FindByCode` mocks.
- Smoke-verified end-to-end against live Postgres: two back-to-back
  POSTs of the same code returned `201 Created` then `409 Conflict`
  with the user-friendly message.

### Phase 6 / Wave 2 Sprint W2-B — currency-create write API + editor UI

Closes the teacher→learner authoring loop opened by W2-A: tenant admins
can mint site-wide currencies and course instructors can mint
course-scoped currencies, all through a single dialog editor reused by
both surfaces. Wallets and topbar pills pick up the new currencies
immediately via the existing `wallet:refresh` window event.

Backend
- New CRUD endpoints on `internal/api/v1/handlers/gamification.go`:
  - Site scope (admin): `POST/PATCH/DELETE /api/v1/gamification/currencies[/:id]`
  - Course scope (instructor): `POST/PATCH/DELETE /api/v1/courses/:course_id/gamification/currencies[/:id]`
  Authorization is enforced by router-level middleware
  (`RequireAdmin` / `RequireInstructor`); the handler reads scope from
  the URL pattern via `resolveScope`.
- Server-side validation: `code` must match `^[a-z][a-z0-9_]{1,31}$`
  (2–32 chars, starts with a letter, lowercase). 2-char minimum
  accommodates the seeded `xp`. `color` must be a 6-digit hex or
  empty. `display_label` required, max 64 chars. `description` max
  500 chars.
- Scope guards: PATCH and DELETE re-load the row and 403 if the
  route-derived scope doesn't match the row's stored
  `(tenant_id, scope_type, scope_id)`. An instructor on course A
  cannot touch a currency on course B or at site scope, even by
  guessing IDs.
- system_owned invariants: POST always sets `system_owned=false`
  (incoming `system_owned: true` is silently ignored). PATCH allows
  every field except `code` (immutable post-create — rules reference
  currencies by code, renaming would break authored content). DELETE
  on a `system_owned=true` row returns 409 Conflict.
- Duplicate detection on POST returns 409 with a useful message
  instead of letting the `uniq_gam_currency_scope_code` unique
  constraint surface as 500.
- Currency-list response (`GET /currencies`) now includes
  `scope_type` + `scope_id` so the frontend can client-side filter
  for the scope it's mounted on.

Also closes a Wave 1 correctness regression that the same bool-default
class (caught for the seeder in W2-A) leaves open in the runtime
`Create` path:
- `internal/repository/postgres/gamification_currency_type.go::Create`
  rewritten to use a raw parameterized INSERT, same pattern as the
  W2-A seed fix. Without it, instructor-created custom currencies
  with `Monotonic: false` or `VisibleInTopbar: false` would silently
  inherit the SQL column DEFAULT TRUE — corrupting spendable
  semantics for shop currencies and violating FERPA visibility for
  instructor-only accounting currencies.
- `Update` already uses `db.Save` which writes all columns including
  zero-valued bools; no change needed there. Verified end-to-end in
  the browser: a fresh "coins" currency edited to flip
  `visible_in_topbar`, `visible_to_student`, and `monotonic` all
  to `false` persists correctly through the PATCH.

Frontend
- `api.gamification.{createCurrency, updateCurrency, deleteCurrency}`
  added to the shared namespace. All three accept an optional
  `{ courseId }` to switch between admin (site) and instructor
  (course) surfaces.
- `<CurrencyEditor>` (Radix dialog) — single component handles both
  create and edit. Form state hydrates from the row on open.
  - Code field is disabled in edit mode regardless of `system_owned`
    (rules reference by code; renaming would break them). System
    rows additionally show a lock icon and helper text
    ("System currency — code is referenced by rules and cannot
    change.").
  - 14-icon palette using the shared `CurrencyIcon` resolver +
    8-swatch color palette + free-form hex input. Custom hex is
    validated client-side against `^(#[0-9A-Fa-f]{6})?$`.
  - Behavior checkboxes (Spendable / Monotonic / Visible to student /
    Show in top bar) with one-line hints.
- `<CurrencyList>` table with create / edit / delete actions. Filters
  rows by scope client-side (so an instructor on course 99 only sees
  course-99 rows). Delete confirms before firing; warns that wallet
  balances are kept (the currency_type_id is still referenced from
  `gamification_wallet_balances`) but the currency stops being
  addressable by name. After every successful write, dispatches
  `wallet:refresh` so mounted `<CurrencyPills>` instances re-fetch.
- New `GamificationCurrenciesPage` mounted at:
  - `/admin/gamification/currencies` (admin site scope)
  - `/courses/:courseId/gamification/currencies` (instructor course scope)
  Scope is inferred from `useParams().courseId` presence.

Tests
- `gamification_test.go` gains 13 new handler tests covering: site/
  course scope create happy paths, bad-code regex, bad-color hex,
  empty-label, duplicate→409, forced `system_owned=false`, rename
  system row (code stays unchanged), toggle `visible_in_topbar=false`
  (pins the bool-default class), scope-mismatch→403, not-found,
  system-owned-delete→409, custom-row-delete→204.
- `CurrencyList.test.jsx` exercises scope filtering, system-owned
  delete-button-disabled, the create flow, and that the courseId is
  threaded correctly into the API call.
- Full backend `./internal/...` suite green. Frontend: 96 tests pass
  (up from 91; +5 for `CurrencyList.test.jsx`).

Out of scope for W2-B (deferred to later sprints):
- Per-learner leaderboard opt-out — Sprint W2-C.
- Badge schema + effect + admin UI — Sprint W2-D.
- Recipe builder UI — Sprint W2-E.

### Phase 6 / Wave 2 Sprint W2-A — top-bar currency pills + wallet drawer

Wave 2 opens with the smallest-viable visible loop: a horizontal
subheader strip above `<main>` that renders the signed-in user's
topbar currencies as pill buttons, and a right-slide-out
`WalletDrawer` that shows per-currency transaction history. Pure
read-side; no schema changes. Locks in the design language
(grayscale-friendly tokens, lucide icon resolution, no alpha-wash
backgrounds) for the four UI-heavy sprints that follow.

- **New backend endpoint**
  `GET /api/v1/users/:id/wallet/transactions?currency_type_id=N&page=&per_page=`.
  Self-or-admin auth (mirrors `GetUserWallet`). `currency_type_id` is
  required and validated; `per_page` clamps to `[1, 100]` with a default
  of `20`. Resolves to
  `GamificationWalletRepository.ListTransactionsForUserAndCurrency`
  — a new repo method that narrows the existing ledger query to a
  single currency. Powers the drawer's per-currency tab without
  over-fetching when a user has years of cross-currency history.
- **`walletBalanceJSON.currency_type_id` now exposed** on the existing
  `GET /users/:id/wallet` response so the drawer can fetch
  unambiguously. Codes can repeat across scopes (e.g., two
  course-scoped "coins"); IDs disambiguate.
- **Frontend `gamificationApi` namespace** added to
  `web/src/services/api.js` (nested under `api.gamification`): wallet
  read, wallet transactions, and currencies list. Future Wave 2
  sprints extend the same namespace for rules/badges/CRUD.
- **`<CurrencyPills>`** in `web/src/components/gamification/` —
  fetches the caller's wallet on mount, filters to
  `visible_in_topbar=true`, sorts by `display_order`, renders pills as
  icon+balance buttons with `title` tooltips. Large balances
  compact-format (`12.4k`, `150k`). Subscribes to a
  `wallet:refresh` window event for future writes to ping.
  `currencyIcon.jsx` resolves the seed's `icon` field — a lucide name
  (`zap`/`gem`/`target`/`shield-check`) → emoji glyph → `Sparkles`
  fallback. Grayscale-eink-friendly: no tinted backgrounds, all
  paper-aesthetic tokens (`surface-1`, `surface-raised`, `text-primary`).
- **`<WalletDrawer>`** — Radix dialog from the right edge, ~28rem
  wide, full height, with the currency's icon + label + balance +
  lifetime-earned in the header and a paginated transaction list in
  the body. `reason` field humanized (`rule:7` → `Rule #7`, `manual:N`
  → `Manual award`, `seed:N` → `Initial grant`, `spend:N` → `Spent: N`).
  "Load more" button appends pages until `total_count` exhausted.
  Honors `motion-reduce`.
- **Layout subheader strip** mounted in both standard mode and 3-5
  mode at the top of the main content column (above `<main>`,
  right-aligned, `h-10 border-b border-surface-raised`). K-2 mode
  intentionally skips pills — K-12 mode defaults game chrome OFF
  for the youngest learners per SYNTHESIS §5. Standard mode hides
  the strip on mobile (`hidden md:flex`) where the hamburger button
  already crowds the top.
- **Tests**: `CurrencyPills.test.jsx` exercises filter-by-topbar,
  display_order ordering, compact balance formatting, drawer-open +
  per-currency transaction fetch, and the `wallet:refresh` event
  refetch. `gamification_test.go` gains six new tests for the
  transactions endpoint (happy path, admin-other-user, forbidden,
  missing/invalid currency_type_id, per_page clamping, repo error).
  Full frontend suite: 91 tests (up from 86); full backend
  `./internal/...` suite green.

Also closes two Wave 1 correctness bugs surfaced by browser-testing
W2-A:

- **`SeedSystemCurrenciesForTenant` bool-default regression**.
  `gorm:"default:..."` tags on the `Monotonic` and `VisibleInTopbar`
  columns caused GORM to elide zero-valued bool inserts in favor of
  the SQL DEFAULT TRUE. The seed declared `mastery_points.VisibleInTopbar=false`
  and `gems.Monotonic=false`, but every tenant came up with
  `mastery_points` *in* the topbar (FERPA breach per SYNTHESIS §2) and
  `gems` flagged as monotonic (breaks spendable semantics — gems are
  meant to decrease on shop spends). Rewrote the seed to use a raw
  parameterized INSERT with explicit columns and ON CONFLICT DO
  NOTHING — every column written every time. The
  `TestSeedSystemCurrenciesForTenant` integration test now pins the
  full {`Spendable`, `Monotonic`, `VisibleInTopbar`} matrix for all
  four system currencies so this regression class can't sneak back.
- **Migration 000039** backfills any tenant rows already written
  through the buggy path: flips `mastery_points.visible_in_topbar` and
  `gems.monotonic` back to `FALSE` where they're currently `TRUE` on
  system-owned rows. Idempotent against post-fix tenants. Down
  migration is intentionally empty (re-creating the bug would
  re-create a compliance violation).

Out of scope for W2-A (deferred to W2-B or later):
- Currency-create write API — Sprint W2-B.
- Animation on balance change (subtle pulse on increment) — deferred
  until W2-B lands so the design language settles first.
- `/wallet` deep-history route — drawer's "Load more" is sufficient
  for Wave 2; route-level view added when the first user complains.

### Phase 6 / Wave 1 Sprint D-3 — correctness finalize (UNIQUE + FERPA seed + flag derivation)

Closes Wave 1. Three correctness wins land together, all behind
migrations or guarded behind the FERPA tag lookup.

- **Migration 000037 — `UNIQUE` constraint on `learning_outcome_results
  (user_id, learning_outcome_id, associated_asset_type,
  associated_asset_id)`**. Closes the residual INSERT-side mastery
  race that PR #10's CHANGELOG documented as outstanding. The migration
  defensively deduplicates any pre-existing dupes (keep most recent;
  tie-break on lower id) before adding the constraint, so it applies
  cleanly against a non-empty prod table.
- **`LearningOutcomeResultRepository.Upsert` reshaped to use
  `INSERT … ON CONFLICT DO NOTHING`**. The loser of a concurrent
  first-time write sees `RowsAffected = 0`, re-fetches under the row
  lock, and falls through to the update path — observing the
  just-inserted row as its "prior" state. Two concurrent
  `CreateResult` calls on the same composite now both produce exactly
  one `OnMasteryCrossed` fire (or zero), never two.
- **Migration 000038 — seed `gamification_ferpa_field_tags`**. The
  table was previously empty in prod, so the FERPA guard had no rules
  to enforce. This migration seeds policy classifications for every
  result/context field shape the seven live emit verbs produce
  (graded submission, completed quiz, enrolled course, viewed page,
  posted discussion entry, mastered outcome, assessed rubric).
  Scores, percents, mastery flags, and per-criterion rubric ratings
  are tagged `education_record`. Course / enrollment / assessor
  identity references are `directory_information`. Workflow state and
  calc methods are `instructor_metadata`. Internal record IDs are
  `non_PII`.
- **`gamification.DerivePolicyFlags`** (new function) wired as the
  first step of `Emitter.Emit`. Walks the event's result/context
  against the tag table and appends `ferpa_protected` +
  `education_record` to `PolicyFlags` whenever an
  education-record-tagged field is present. Idempotent. Means internal
  emit call-sites never need to set policy flags manually — the FERPA
  classification flows from the seeded tags into the persisted event
  row, where downstream policy queries can trust it.
- **`CheckFerpa` is now a backstop, not a hot path**. After
  derivation, the guard only fires on hand-built events that bypass
  derivation (e.g., a future external write endpoint). Documented in
  the emitter pipeline godoc.

What this means in practice: a graded outcome now emits an event whose
`policy_flags` contains `{ferpa_protected, education_record}` — making
the row queryable as FERPA-protected at the data-access layer (Wave 2
leaderboards will rely on this).

**Wave 1 is now closed.** The remaining "out of scope" items from
the earlier sprints — `POST /api/v1/gamification/events` write
endpoint, the 13 remaining trigger verbs — are deferred to a future
"Wave 1 extras" PR or until consumed by Wave 2 features. The pgvector
CI matrix was already in place since Sprint A; no change needed.

### Phase 6 / Wave 1 Sprint D-2 — discussion + outcome mastery + rubric emit wiring

Lands the three remaining Wave 1 emit verbs. After Sprint D-2 the
in-product gamification engine recognizes every Wave 1 trigger that
SYNTHESIS.md called for: a posted discussion entry, an outcome
mastery transition, and a rubric assessment all flow through the
same dispatcher → predicate → effect path that the Sprint C/D-1
verbs already use.

- **`internal/service/gamification/vocabulary.go`**: adds
  `VerbPosted`, `VerbAssessed`, `ObjectDiscussionEntry`,
  `ObjectRubric` to the canonical constants. `VerbMastered` and
  `ObjectOutcome` were already present from Sprint C.
- **Callback infrastructure on three more services**:
  `DiscussionService.OnEntryCreated`,
  `RubricService.OnAssessmentCreated`,
  `LearningOutcomeService.OnMasteryCrossed`. Same goroutine fan-out
  with panic recovery as the D-1 callbacks; failures never block the
  originating write.
- **Per-row mastery transition guard in `LearningOutcomeService`**:
  `LearningOutcomeResultRepository.Upsert` now returns the atomic
  pre-write mastery value `(priorMastery *bool, err error)` and runs
  its read-then-write inside a single transaction with
  `SELECT … FOR UPDATE` on the existing row. This serializes
  concurrent writes to the same
  `(user_id, learning_outcome_id, asset_type, asset_id)` composite
  and lets `CreateResult` fire `OnMasteryCrossed` only on the
  false/nil → true transition without the check-then-act race two
  separate roundtrips would have introduced. Rollup-level mastery
  is still left to the `OutcomeMastery` predicate. The residual
  INSERT-side race (two concurrent first-time writes both finding
  no row) is left to a Sprint D-3 migration that adds a UNIQUE
  index on the composite + `ON CONFLICT` semantics.
- **`internal/service/gamification/wiring/`** (three new files):
  `DiscussionEntryPostedEmitCallback`,
  `OutcomeMasteryCrossedEmitCallback`,
  `RubricAssessmentCreatedEmitCallback`. Each mirrors the Sprint D-1
  pattern: load the entity, walk to its course / outcome / rubric for
  the tenant_id, emit a snake-case-keyed xAPI-shaped event. Errors
  are logged via `slog.Error` and swallowed.
- **`cmd/server/main.go`**: registers the three new callbacks
  against `discussionService`, `rubricService`, and
  `learningOutcomeService` in a Sprint D-2 block alongside the
  existing D-1 wiring.
- **End-to-end test**:
  `TestCreateResult_TriggersMasteryRuleOnFirstTransitionOnly` —
  exercises the full path through `LearningOutcomeService.CreateResult`
  → service-level transition detection → callback → wiring emit →
  rule dispatch → AwardCurrency effect → wallet ledger. Asserts
  exactly one wallet transaction across two CreateResult calls (the
  second is mastered-to-mastered, so it must NOT re-emit).

Out of scope for D-2 (Sprint D-3 targets):
`POST /api/v1/gamification/events` write endpoint, full pgvector
CI matrix, policy-flag derivation, seeded FERPA tag rows.

### Phase 6 / Wave 1 Sprint D-1 — emit wiring + read-side API

Wires the Sprint C rules engine into the rest of Paper LMS. Internal
service-layer events (graded submissions, completed quizzes, course
enrollments, page views) now fire `gamification.Emit` via async
callback hooks, so any rule a teacher authors against those triggers
actually fires in production. Adds the first slice of the read-side
HTTP API (wallet + currencies) so a learner / admin can see engine
state from the browser.

- **`internal/service/gamification/vocabulary.go`**: canonical
  `Verb*` and `Object*` constants (submitted, graded, completed,
  viewed, enrolled / Assignment, Submission, Quiz, Page, Course, …).
  Rules reference these strings directly; one source of truth so a
  call-site and a rule can't drift.
- **Callback infrastructure on three more services**: `QuizService`,
  `EnrollmentService`, and the new `ContentViewService` (thin
  orchestrator owning the `content_views` upsert) all gained the
  `OnX(cb)` / `fireOnX(...)` pattern the existing
  `SubmissionService.OnGraded` introduced. Goroutine fan-out with
  panic recovery; failures NEVER block the originating write.
- **`internal/service/gamification/wiring/`** (new package):
  one wiring function per emit domain — `GradedSubmissionEmitCallback`,
  `CompletedQuizEmitCallback`, `EnrolledCourseEmitCallback`,
  `ViewedContentEmitCallback`. Each returns a typed callback closed
  over the right repositories, walks the entity → course → account
  chain for `tenant_id`, builds the xAPI-shaped event, and calls
  `Emit`. Errors are logged via `slog.Error` and propagation is
  swallowed by design (a gamification failure can't break the
  student's submission write).
- **`internal/api/v1/handlers/pages.go`**: the authenticated
  `GetPage` handler calls `contentViewService.RecordView` after
  rendering, upserting `content_views` and firing the
  `verb=viewed, object_type=Page` callback. The anonymous
  `GetPublicPage` path is intentionally untouched (no `user_id` in
  Locals).
- **`internal/api/v1/handlers/gamification.go`** (new):
  `GET /api/v1/users/:id/wallet` — joined wallet balance + currency
  metadata view (self-or-admin-authorized), and
  `GET /api/v1/gamification/currencies` (with `?topbar_only=true`) —
  every currency_type the tenant has defined, sorted by
  `display_order`. These are the read endpoints the topbar widget
  and learner profile will consume.
- **`cmd/server/main.go`**: assembles the `Emitter` against every
  Sprint C repository, registers each wiring callback against the
  service that owns the lifecycle event, wires
  `contentViewService` into the page handler via
  `SetContentViewService`, and threads the new
  `GamificationHandler` through `NewRouter` so the read API is
  reachable.
- **End-to-end test**:
  `TestGradeSubmission_TriggersRuleViaCallback` — exercises the full
  production path. Builds `SubmissionService` with the real
  callback, calls `SubmissionService.Grade(95)`, polls until the
  downstream wallet transaction lands (the callback fires
  asynchronously in a goroutine), asserts +50 xp + a single
  `rule_evaluation` row. This is the proof that *all* the Sprint A
  → B → C → D-1 pieces snap together.

Out of scope for Sprint D-1 (Sprint D-2 follow-up): discussion entry
emit, outcome-mastery threshold-crossing emit, rubric assessment emit,
`POST /api/v1/gamification/events` write endpoint, full pgvector CI
matrix, and the policy-flag derivation refinement (the FERPA guard's
"no tag → no violation" posture means current emits are safe with
empty policy_flags).

### Phase 6 / Wave 1 Sprint C — rule engine end-to-end

Lands the working rules engine: a public `gamification.Emit` entry point
that takes one xAPI-shaped event, runs a FERPA pre-flight, persists the
event, and dispatches every matching enabled rule through predicate
evaluation, cooldown / max-per-window guards, and effect execution. The
predicate vocabulary, mastery calculators, `AwardCurrency` effect, and
system-currency seeder from prior Sprint B PRs are now consumed end-to-end.

Highlights:

- **Migration 000036 (`content_views`)**: per-user content-view
  aggregates. Atomic upsert via `ON CONFLICT` for view-count and total-
  seconds increments. The `ViewedContent` predicate now reads real
  view counts and cumulative-seconds totals through the new
  `ContentViewRepository`, supporting "block lesson progression until
  user views page X at least N times" rule patterns.
- **`Predicate.Needs()`**: every predicate declares which slices of
  `ActorSnapshot` (and which IDs within them) it touches at evaluation
  time. The snapshot loader unions Needs across a rule's full
  condition_set tree and issues one targeted query per slice rather
  than full-user dumps. `ConditionSet.Needs()` unions children.
- **Snapshot loader** (`snapshot.go`): hydrates `ActorSnapshot` for one
  user against a `Needs` declaration. Wallet hydration is triggered by
  any currency-code dependency so `CurrencyThreshold` can resolve
  codes. Wave 1 does not populate Enrollments / LastLogin (no
  repository yet); predicates that need them fail-with-reason rather
  than crash.
- **Predicate + effect factories**: `predicates.DecodePredicate` and
  `effects.DecodeEffect` / `DecodeEffects` parse `gamification_rules`
  JSONB into typed values. Recursive ConditionSet decode + per-kind
  validation (non-zero IDs, valid `MinLevel`, `Op` membership,
  `N_OF_M` requires `Threshold > 0`).
- **Rule index** (`rule_index.go`): `BuildRuleIndex` parses each rule's
  `trigger_event` JSONB once and buckets rules by `(verb, object_type)`
  for OnEvent, by handle for OnManualTrigger, and a flat list for
  OnSchedule. Malformed-trigger rules land in `Skipped()` so observable
  without blocking other rules.
- **Cooldown + max-per-window guard** (`cooldown.go`):
  `CheckCooldown(ctx, repo, rule, userID, now)` enforces both gates via
  the new `LastFiringForUserRule` and `CountFiringsInWindow` repo
  methods. Rolling 24h / 7d / lifetime windows. Unknown window value
  returns an error (rule is misconfigured) rather than silently
  allowing.
- **FERPA guard** (`ferpa_guard.go`): pre-flight on `Emit` that
  cross-checks every `result` / `context` field path against the
  `gamification_ferpa_field_tags` lookup for the event's
  `object_type`. Rejects emits where an `education_record`-classified
  field appears without both `ferpa_protected` and `education_record`
  policy flags. Wave 1 enforces only `education_record`; other
  classifications are advisory.
- **Dispatcher** (`dispatcher.go`): per-rule pipeline. Decodes
  condition_set + effects, checks cooldown, hydrates snapshot,
  evaluates predicates, runs effects with **stop-on-first-error**
  semantics (prior successes stay durable in the wallet ledger,
  subsequent effects are recorded as skipped in `effects_fired`).
  Writes a `gamification_rule_evaluations` row per rule fire with the
  full predicate trace and effect outcome list.
- **Emitter** (`emitter.go`): single public `Emit(ctx, event)` entry
  point. FERPA pre-flight → persist event → rebuild rule index per
  emit at site scope → dispatch. Returns `EmitResult` with the
  persisted event ID + the dispatch outcome.
- **End-to-end DB integration tests**: 5 tests against a real `pgvector/pg16`
  Postgres prove the full pipeline:
  - Award-XP-on-assignment happy path (event → rule fires → wallet tx
    of +50 → balance = 50)
  - Non-matching trigger (zero rules considered)
  - Predicate false (eval row with `result=false`, no wallet tx)
  - Cooldown enforcement (second emit blocked, no second wallet tx)
  - System reputation currency invariant

Sprint D scope (task 12 + 13 + 14): xAPI emission from existing
submission / quiz / lesson / course handlers (the 20 core triggers), API
handlers (`POST /api/v1/gamification/events`, `GET /currencies`,
`GET /wallet`), full integration-test pass against `pgvector/pgvector:pg16`,
and the content-view emission middleware that increments `content_views`
on every page render.

### Phase 6 / Wave 1 — gamification foundations

First load-bearing slice of the unified gamification engine. Scaffolding
only — no teacher UI, no rules dispatch, no badge issuance yet. See
`docs/research/gamification-2026-05/PHASE6-WAVE1-PLAN.md` for the full
plan; this PR delivers tasks 2–5.

- **Migration 000032 (`gamification_event_bus`)**: xAPI-shaped event store
  (`gamification_events`) with indexes for actor/verb/object/tenant
  lookups and a unique partial index on `(source, source_event_id)` for
  idempotent ingest. Every gamification-relevant action will eventually
  emit one row here.
- **Migration 000033 (`gamification_rules`)**: the rules table plus
  `gamification_rule_evaluations` for the audit trail. Defines the
  `gamification_scope_type` and `gamification_audience` Postgres enums
  reused by later migrations. `(rule_id, user_id, evaluated_at)` is
  uniquely indexed so same-microsecond duplicate evaluations surface as
  errors rather than silently double-firing.
- **Migration 000034 (`gamification_currencies_and_wallet`)**: the
  MyCred-style `gamification_currency_types` table (user-defined
  currencies per tenant/course/section) plus `gamification_wallet_balances`
  and the immutable `gamification_wallet_transactions` ledger. Four
  system-owned currencies (xp, gems, mastery_points, reputation) will be
  seeded by the Go-side seeder in a later PR.
- **Migration 000035 (`tenant_mode_and_ferpa_tags`)**: adds
  `accounts.tenant_mode` and `accounts.coppa_strict`, plus
  `gamification_ferpa_field_tags` for the (object_type, field_path) →
  FERPA classification lookup that the FERPA guard will enforce.

Together these unlock the rules engine, the predicate evaluator (Wave 1
task 10), the AwardCurrency effect (task 8), the system-currency seeder
(task 9), and xAPI emission across 20 core triggers (task 12).

Also in this PR:

- GORM models for the seven new tables, registered with `AutoMigrate`
  and aligned column-for-column with the SQL chain so
  `TestSchemaParity_Wave1` stays green.
- Five new repositories (`GamificationEventRepository`,
  `GamificationRuleRepository`, `GamificationCurrencyTypeRepository`,
  `GamificationWalletRepository`, `GamificationFerpaFieldTagRepository`).
  The wallet repo's `ApplyTransaction` is the single atomic mutation
  primitive — appends the ledger row + updates the balance row under a
  `SELECT … FOR UPDATE` lock.
- `internal/service/gamification/predicates/`: the `Predicate` interface,
  `ActorSnapshot`, `Trace`, recursive `ConditionSet` with AND / OR /
  N_OF_M short-circuit semantics, and the first end-to-end predicate
  (`SubmittedAssignment`). The remaining six Wave 1 predicates follow in
  a later PR.
- `internal/service/gamification/mastery/`: the six mastery `calc_method`
  algorithms as zero-value stubs (Khan spaced-retrieval, decaying
  average, most-recent, highest, n-times, weighted-average). Real
  implementations land in the PR for Wave 1 task 7.

Wave 1 remaining tasks (per the plan): predicates 2–7, full mastery
calculators, AwardCurrency effect, system-currency seeder, rule dispatch
loop, FERPA guard, xAPI emission hooks into existing services, API
handlers, integration tests against `pgvector/pgvector:pg16`.

### Phase 6 / Wave 1 — predicate vocabulary + mastery + AwardCurrency + seeder

Sprint B of Wave 1. Lands four tasks (6, 7, 8, 9) from
`docs/research/gamification-2026-05/PHASE6-WAVE1-PLAN.md` — no migrations,
all Go-only.

- **Six new predicates** in `internal/service/gamification/predicates/`:
  `SubmittedQuiz`, `ViewedContent`, `OutcomeMastery`, `CurrencyThreshold`,
  `EarnedBadge`, `ReputationThreshold`. Plus a `ViewedContent` map field
  on `ActorSnapshot`. Each predicate ships with table-driven tests; the
  vocabulary now covers Submission / Content / Mastery / Economy / Badge
  with the room for Enrollment, Time, and Discussion predicates in later
  sprints. `ReputationThreshold` is a thin wrapper that delegates to
  `CurrencyThreshold` with `code="reputation"` so rule authors don't
  repeat the literal.
- **Six real mastery `calc_method` implementations** in
  `internal/service/gamification/mastery/`: `most_recent`, `highest`,
  `weighted_average`, `n_times`, `decaying_average`, and
  `khan_spaced_retrieval`. Khan uses a real half-life
  (`score · 2^(-Δdays/halfLife)`) with defaults 7-day half-life and 0.8
  reattempt threshold. A shared `level_discretizer.go` maps the
  continuous `Value` to `novice|familiar|proficient|mastered` so all six
  agree on bucketing. `Params` grew a `Now` field so Khan can compute
  final decay against arbitrary evaluation times.
- **`AwardCurrency` effect** in a new
  `internal/service/gamification/effects/` package, together with the
  `Effect` interface, `EffectDeps`, `TriggeringContext`, and
  `EffectResult` shapes that every future effect will implement
  (`AwardBadge`, `ReleaseContent`, `BranchPath`, `UnlockCapability`,
  `Notify`, `AdvanceRankOrLevel`, `EnrollInGroup`). `AwardCurrency`
  resolves currencies by code via a scope-walking
  `ResolveCurrencyByCode` helper (Wave 1 walk: requested scope →
  site-fallback), applies an optional multiplier, and writes the
  triggering rule/event IDs + FERPA policy flags onto the wallet
  transaction.
- **System-currency seeder + backfill binary**: a new
  `internal/service/gamification/seed.go` exports
  `SeedSystemCurrenciesForTenant`, idempotent via
  `clause.OnConflict{DoNothing: true}` against the
  `uniq_gam_currency_scope_code` index from migration 000034.
  `cmd/seedgamification` is a one-shot CLI that lists every `accounts`
  row and seeds each. `cmd/server/main.go` calls the seeder on every
  boot right after `db.SeedDefaultAccount`, so new tenants always have
  the four system currencies (xp, gems, mastery_points, reputation).
  Integration tests against the dev Postgres prove single-tenant,
  multi-tenant (3 tenants → 12 rows), and idempotent re-run behavior.

Wave 1 remaining tasks after this PR: 10 (`gamification.Emitter` +
rule dispatch loop), 11 (FERPA guard on Emit), 12 (xAPI emission from
existing submission/quiz/lesson/course services), 13 (API handlers for
events / currencies / wallet), 14 (full integration test pass against
`pgvector/pgvector:pg16`).

## [0.1.0] — 2026-05-11

Initial public release.

### Highlights

- Canvas REST API compatibility across 360 routes, organized in 60 handlers.
- 84 GORM models covering courses, modules, assignments, quizzes,
  discussions, gradebook, rubrics, learning outcomes, and SIS data.
- React 18 + React Router 7 frontend, 67 pages, 40 lazy-loaded chunks.
- Auth: JWT cookies, OAuth 2.0, Personal Access Tokens, SAML 2.0, LDAP, CAS.
- Storage: pluggable local disk / S3 / MinIO / Cloudflare R2 backends.
- LTI 1.3 platform (OIDC, AGS, NRPS, Deep Linking).
- IMSCC / Common Cartridge 1.3 import + export for migration in/out of
  Canvas, Schoology, Moodle.
- OneRoster v1.2 SIS sync; Canvas SIS Imports CSV format.
- K-12 differentiators: K-2 picture-cue mode, parent observer accounts,
  pairing codes, weekly digest emails.
- Accessibility: WCAG 2.1 AA, reading preferences (dyslexia-friendly fonts,
  spacing, italic-stripping, TTS toggle), self-hosted OpenDyslexic / Lexend
  / Atkinson Hyperlegible.
- Mobile-first PWA with offline support.

[Unreleased]: https://github.com/EduThemes/paper-lms/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EduThemes/paper-lms/releases/tag/v0.1.0
