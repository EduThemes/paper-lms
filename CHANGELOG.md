# Changelog

All notable changes to Paper LMS are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
