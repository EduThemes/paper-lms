# Changelog

All notable changes to Paper LMS are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
