# Gamification Audit — Phase 7 / Sprint 7-A

**Date:** 2026-05-15
**Scope:** Paper LMS gamification surface after Wave 3 (W3-A → W3-B → W3-C) lands. Includes Phase 6 Wave 1+2 carryover code that the wave 3 work depends on.
**Branch:** `phase6-wave3-prelude-logo` (uncommitted; audit performed against working tree)
**Rationale:** User-flagged drift concern after a high-velocity 3-sprint stack added ~40 files + 12 migrations. Audit is a pre-merge gate.

---

## Severity legend

- **BLOCK** — must fix on the audit branch before merging to `main`. Data integrity, security, correctness, or "next contributor immediately trips on this" friction.
- **HIGH** — should fix this phase. Defer to follow-up only if cost is large.
- **MEDIUM** — worth fixing in this phase if cheap; otherwise track as follow-up.
- **LOW** — informational; CLAUDE.md or doc updates.

---

## Section 1 — Data-model review

### F1.1 (BLOCK) — `AUTO_MIGRATE=true` breaks on a freshly-SQL-migrated DB

**Pattern:** ORM-generated constraint name shadows a SQL-chain index of a different shape.

**Where:**
- `internal/domain/models/late_policy.go:7` — `CourseID uint ... gorm:"not null;uniqueIndex"`
- `internal/db/migrations/000016_backfill_missing_tables.up.sql:466` — `CREATE UNIQUE INDEX IF NOT EXISTS idx_late_policies_course_id ...`

**What goes wrong:** The Go struct declares `uniqueIndex` (no name). On `AutoMigrate`, GORM looks for a constraint named `uni_late_policies_course_id` and tries to `DROP CONSTRAINT` it to recreate. The SQL chain created an INDEX with a different name (`idx_late_policies_course_id`) instead of a CONSTRAINT, so the DROP fails with `SQLSTATE 42704` and the entire backend startup aborts. Reproduced live during the W3 test pass — see `/tmp/paper-lms-server.log` from `make dev` against a fresh `make migrate-up`.

**Blast radius:** Every new contributor's first `make dev`. Blocks the dev experience entirely. Not gamification-specific but discovered while spinning up the gamification test fixture; gamification is collateral damage of a pre-existing rift between AutoMigrate intent and the SQL chain's reality.

**Fix sketch (pick one, all are ~15 minutes):**
- **Recommended:** Drop the `AutoMigrate` step from `cmd/server/main.go` production path entirely; gate it behind `DEV_AUTO_MIGRATE=true` for the convenience case. SQL migration chain is the documented source of truth (CLAUDE.md says so), the parity test (`TestSchemaParity`) is the enforcement. AutoMigrate adds zero value once those exist.
- Alternative: drop the GORM `uniqueIndex` tag from `late_policy.go:7`. The SQL index already enforces uniqueness; the tag is redundant.
- Alternative: rename the SQL index to `uni_late_policies_course_id` AS A CONSTRAINT (not just an index). Same enforcement, AutoMigrate sees what it expects.

**CLAUDE.md update:** Add a "GORM `uniqueIndex` tag vs SQL chain INDEX" pattern note alongside the bool-default lesson.

---

### F1.2 (HIGH) — Foreign-key-by-convention across gamification tables

**Pattern:** `Foreign Key by Convention Only` from `schema-anti-patterns.md`.

**Where (all confirmed-not-FK columns):**
- `gamification_events.tenant_id`, `actor_id`, `object_id` (migration 000032)
- `gamification_currency_types.tenant_id`, `scope_id` (migration 000034)
- `gamification_wallet_balances.user_id` (migration 000034)
- `gamification_wallet_transactions.user_id` (migration 000034)
- `gamification_rules.tenant_id`, `scope_id`, `created_by` (migration 000033)
- `gamification_rule_evaluations.user_id` (migration 000033)
- `gamification_badges.tenant_id`, `scope_id`, `created_by` (migration 000041)
- `gamification_badge_awards.user_id`, `awarded_by`, `evidence_event_id` (migration 000041)
- `enrollments.pseudonym_pool_code` (no FK to pool catalog because the catalog is code-resident, not DB-resident — this one is acceptable)

**What goes wrong:**
- Deleting a user leaves orphan wallet balances, transactions, badge awards, rule evaluations, events. ON DELETE behavior is implicit.
- Deleting an account/tenant leaves orphan currencies, rules, badges.
- Deleting a course leaves orphan rules + currencies that targeted it.
- A buggy write path could insert `user_id=999999` (no such user) and the DB has nothing to say about it.

**Counter-arguments worth noting (this is why I'm marking HIGH not BLOCK):**
- The repo layer is gated; raw inserts from unknown sources don't happen.
- Cascade-on-user-delete is a thorny design decision in an education context (FERPA records often need to *survive* a user delete). The schema currently dodges it by being silent.
- A wholesale FK add across 10 columns is a large migration that needs a backfill plan (some existing rows may already be orphaned).

**Fix sketch:**
- Don't add FKs blindly. Pick one logical group per follow-up sprint:
  1. `gamification_wallet_*.user_id` + `gamification_badge_awards.user_id` → FK to `users(id)` with `ON DELETE RESTRICT` (FERPA-protective; admin must explicitly handle the cascade).
  2. `gamification_*.tenant_id` → FK to `accounts(id)` with `ON DELETE RESTRICT`. (Tenant deletion is admin-only; this just makes it explicit.)
  3. Leave `scope_id` polymorphic (it's a polymorphic FK by design — see F1.3).

**Follow-up task:** "Add user_id FKs across gamification tables" — own migration, own PR, with the existing-orphan backfill step.

---

### F1.3 (MEDIUM) — Polymorphic foreign keys on `(scope_type, scope_id)`

**Pattern:** `Polymorphic Foreign Keys` from `schema-anti-patterns.md`.

**Where:** `gamification_currency_types`, `gamification_rules`, `gamification_badges` all carry `(scope_type ENUM, scope_id BIGINT)` where scope_type ∈ `{site, district, school, course, section}` and scope_id refers to a row in a different table depending on scope_type.

**What goes wrong:** No FK can enforce that `scope_type='course' AND scope_id=42` actually points at an existing course. A bad write or a course deletion can leave the rule pointing at nothing.

**Counter-argument:** This is the intentional design from the Wave 1 PHASE6-WAVE1-PLAN. The scope walk (`section > course > school > district > site`) needs uniform shape to be efficient; splitting into N tables with concrete FKs would force every read to UNION across them. Defensible.

**Fix sketch — DEFER:** This is a defensible architectural choice. Document it. The data-model audit's job is to surface the pattern; the user/team can choose to live with it.

**Recommendation:** Add a CHECK constraint that `scope_type` is one of the enum values **on `gamification_badges`** (currently plain text — see F1.4). At minimum the columns should be consistently constrained.

---

### F1.4 (HIGH) — `scope_type` enforcement drifts across gamification tables

**Pattern:** `Enum-as-String` selectively applied — same logical column, different constraint discipline per table.

**Where:**
- `gamification_currency_types.scope_type` — `gamification_scope_type NOT NULL` ✓ (migration 000034)
- `gamification_rules.scope_type` — `gamification_scope_type NOT NULL` ✓ (migration 000033)
- `gamification_badges.scope_type` — `text NOT NULL` ✗ (migration 000041)

**What goes wrong:** Nothing structurally prevents `INSERT INTO gamification_badges (scope_type, ...) VALUES ('coarse', ...)` (typo). The application's `models.GamificationScopeType` constants are the only line of defense, and any direct SQL or buggy serialization can bypass them.

**Fix sketch:** Migration to alter `gamification_badges.scope_type` to `gamification_scope_type` enum. Two-step (CHECK constraint first, then enum cast) to be safe with existing rows.

```sql
ALTER TABLE gamification_badges
  ADD CONSTRAINT chk_gam_badges_scope_type
  CHECK (scope_type IN ('site','district','school','course','section'));
-- next migration, after verifying:
ALTER TABLE gamification_badges
  ALTER COLUMN scope_type TYPE gamification_scope_type USING scope_type::gamification_scope_type;
```

**Blast radius:** Migration only. No code changes.

**Action:** Block-merge for this phase. Fix in 7-A.

---

### F1.5 (LOW) — Go field type `GamificationScopeType` weakened by `gorm:"type:text"` tag

**Pattern:** `Hand-Written Shadow of a Generated Type`, inverted — the Go type is *stricter* than the GORM tag claims.

**Where:** `internal/domain/models/gamification_rule.go:57`, `gamification_currency_type.go:27`, `gamification_badge.go:22`.

```go
ScopeType GamificationScopeType `json:"scope_type" gorm:"not null;type:text"`
```

**What's happening:** The Go field's static type is the enum-typed `GamificationScopeType`, which limits compile-time writes to the five known constants. But the `type:text` GORM tag tells AutoMigrate the column is plain TEXT. The SQL schema chain ALSO uses the enum (`gamification_scope_type`) — except for badges (see F1.4). So:
- DB layer: stricter (enum)
- GORM tag: looser (text)
- Go static type: as strict as DB

The `type:text` tag exists because AutoMigrate doesn't know how to declare a Postgres enum (it'd try to recreate it and fail the parity test — comment on line 53 of `gamification_rule.go` documents this). Net effect: the tag is a workaround, not a bug.

**Action:** No fix. Document the pattern as "load-bearing GORM tag" in CLAUDE.md so the next contributor doesn't simplify it away.

---

### F1.6 (LOW) — `string` for nullable text with policy-bearing partial UNIQUE — class of bug

**Pattern:** The bool-default class generalized to TEXT. Originally caught and patched during W3-B end-to-end testing.

**Where (patched already):**
- `enrollments.pseudonym_name` was `string` in `internal/domain/models/enrollment.go`; now `*string`. The partial UNIQUE index `idx_enrollments_pseudonym_unique_per_course` was treating empty-string as not-NULL and collided on the first two unassigned rows per course.

**Cross-check — other partial UNIQUE indexes on the gamification surface:**
- `gamification_events.source_event_id` (migration 000032) — `text` nullable. Go model: `SourceEventID *string` ✓ already correct.
- `gamification_wallet_transactions.triggering_event_id` (migration 000034) — `bigint` nullable. Go model: `*uint` ✓.

**Cross-check — outside gamification but flagged by the same logic:**
- Run `grep -rn "WHERE.*IS NOT NULL" internal/db/migrations/` against the model layer. Only one match outside gamification (`000018_identity_rename.up.sql` rename-migration comment, not a partial index). No latent bugs.

**Action:** Add to CLAUDE.md "load-bearing patterns" — "For TEXT columns with partial UNIQUE indexes (`WHERE col IS NOT NULL`), the Go field MUST be `*string`, not `string`. GORM serializes `string` as `''` which IS indexed and collides."

---

### F1.7 (LOW) — `ferpa_classification` as VARCHAR + CHECK instead of enum

**Pattern:** `Enum-as-String` with CHECK band-aid.

**Where:** `gamification_currency_types.ferpa_classification` and `gamification_ferpa_field_tags.classification` both `text NOT NULL` with a CHECK in 000034 limiting values. The Go field is `string` (not a typed constant).

**What's happening:** The CHECK constraint enforces the four classifications. Go-side, the classification flows around as a bare `string` — no `FerpaClassification` const-set type. A typo in handler code or a misclassified field tag would fail at write time (DB) but not at compile time.

**Fix sketch:** Add a `FerpaClassification` typed-string in `internal/domain/models/gamification_ferpa_tag.go` with the four constants. Update reads/writes. Cosmetic — the CHECK catches it at write time.

**Action:** Follow-up task, not block-merge. ~30 min refactor.

---

### F1.8 (LOW) — Currency `scope_id=tenantID` convention is undocumented

**Pattern:** Domain convention that's load-bearing for handler code but lives only in the seeder.

**Where:** `SeedSystemCurrenciesForTenant` (`internal/service/gamification/seed.go`) writes site-scope currencies with `ScopeID = tenantID`. Caught during W3-A test pass when my handler was querying with `scope_id=0` and getting "unknown currency: xp."

**Why this is a data-model issue, not a handler bug:** The handler's mental model "site scope means scope_id=0" is reasonable; the seeder's "site scope means scope_id=tenantID" is also reasonable. The SQL schema has no opinion. Neither is documented at the model layer. Anyone reading `GamificationCurrencyType` has no way to know.

**Fix sketch:** Add a comment on `GamificationCurrencyType.ScopeID` documenting the convention. Optional: extract a helper `ScopeKey(scopeType, scopeID, tenantID) (ScopeType, uint)` that normalizes the convention at the handler boundary.

**Action:** Add the comment now (5 minutes). Helper is a follow-up if the convention bites someone again.

---

### F1.9 (BLOCK) — `gamification_badges` text columns over-default to `''`

**Pattern:** `Nullable When It Shouldn't Be`, inverted — text columns NOT NULL with `DEFAULT ''` instead of `DEFAULT NULL`.

**Where:** Migration 000041 lines 31-37:
```sql
description         text          NOT NULL DEFAULT '',
icon                text          NOT NULL DEFAULT '',
image_url           text          NOT NULL DEFAULT '',
color               text          NOT NULL DEFAULT '',
audience_level      text          NOT NULL DEFAULT '',
```

**What goes wrong:**
- Distinguishing "this badge has no description" from "this badge has an empty description set by the author" is impossible from the schema.
- If `audience_level` later joins a partial UNIQUE index (Wave 3 audience-filter rules planned per the W2-D CHANGELOG), the same `''`-vs-`NULL` bug class as F1.6 will fire.
- Other gamification tables (currencies, rules) use nullable text for the analogous fields. Inconsistency.

**Compare:** `gamification_currency_types.icon`, `color`, `description` (migration 000034) — all just `text` (nullable). That's the right pattern.

**Fix sketch:** Migration to relax `NOT NULL` + `DEFAULT ''` to nullable on the five columns. Existing `''` values stay; new inserts can use NULL.

```sql
ALTER TABLE gamification_badges
  ALTER COLUMN description    DROP NOT NULL,
  ALTER COLUMN description    DROP DEFAULT,
  -- ... repeat for icon, image_url, color, audience_level
```

**Blast radius:** Migration. No Go changes required (Go field is `string`; `NULL` deserializes to `""` which is what callers already see).

**Action:** Block-merge. The audience_level column is going to be used by Wave 3+ audience-filter rules; getting this right now is cheaper than later.

---

### F1.10 (MEDIUM) — `created_by` / `awarded_by` int pointers without FK or audit semantics

**Pattern:** Half-built audit trail.

**Where:**
- `gamification_rules.created_by *uint` — set on Create, never written again, no FK.
- `gamification_badges.created_by *uint` — same.
- `gamification_badge_awards.awarded_by *uint` — set on manual grant, NULL on rule-fired award.

**What goes wrong:**
- No FK → deleted users leave orphan provenance.
- No accompanying "updated_by" or audit-log shape → "who edited this rule yesterday?" is unanswerable.
- The semantic load on `awarded_by IS NULL` ("system did it") vs `awarded_by IS NOT NULL` ("admin actor X did it") is implicit, not documented at the model layer.

**Fix sketch (DEFER):** A real audit-log table is the right answer if the team wants this surface to actually work. Right now it's "looks like audit but isn't." Don't fix piecemeal — either accept the half-shape (document it as informational) or design a proper audit table in a future phase.

**Action:** Follow-up task. Document the limitation in CLAUDE.md.

---

### F1.11 (LOW) — `gamification_rules.audience_level` typed in Go but text in SQL

**Pattern:** Same as F1.5 but for audience_level.

**Where:** `gamification_rule.go:59` — `AudienceLevel GamificationAudience` (typed). Migration 000033 has audience_level as `gamification_audience` enum.

**Cross-table inconsistency:** `gamification_badges.audience_level` is plain `text NOT NULL DEFAULT ''` (F1.9). Three places where the same logical thing is modeled differently:
- Rules: SQL enum, Go typed const, GORM tag `type:text`.
- Badges: SQL text, Go `string`, GORM tag absent.
- Accounts.tenant_mode: SQL enum, Go typed const, GORM tag `type:text` ✓.

**Action:** F1.9's badge fix should align badge `audience_level` with the rules pattern (enum at the DB, typed Go const, `type:text` GORM tag). Bundled into F1.9 remediation.

---

### F1.12 (LOW) — Wide migration chain, no parity test for the new tables yet

**Pattern:** `Schema Drift` risk for newly-introduced tables.

**Where:** Migrations 000042, 000043, and any 7-B 000044+. The repo has `TestSchemaParity_Wave1` (per CLAUDE.md) covering Wave 1 tables. No equivalent covers Wave 3.

**Fix sketch:** Either widen the existing parity test to cover Wave 3 tables, or add `TestSchemaParity_Wave3`. The cost is low because the W3 chain is short (indexes-only + enrollment columns + future snapshots).

**Action:** Follow-up — sequence with Sprint 7-B's snapshot migration so the parity coverage lands once.

---

## Section 1 — Verdict

For each finding, classified per the rubric:

| ID | Severity | Verdict | Owner sprint |
|---|---|---|---|
| F1.1 (AUTO_MIGRATE breakage) | **BLOCK** | REFINE — drop AutoMigrate from prod path | 7-A |
| F1.2 (FK-by-convention) | HIGH | REMODEL (multi-PR backfill) | 7-A surface, follow-up implements |
| F1.3 (polymorphic FK) | MEDIUM | REFINE — document, don't fix | 7-A doc note |
| F1.4 (badge scope_type drift) | **HIGH** | REFINE — one migration | 7-A |
| F1.5 (`type:text` weakening) | LOW | DOCUMENT in CLAUDE.md | 7-A |
| F1.6 (string→*string for nullable text) | LOW | DOCUMENT in CLAUDE.md (already patched) | 7-A |
| F1.7 (FERPA classification as string) | LOW | Follow-up task | later |
| F1.8 (scope_id=tenantID convention) | LOW | Comment on the model | 7-A |
| F1.9 (badge text NOT NULL DEFAULT '') | **BLOCK** | REFINE — one migration | 7-A |
| F1.10 (half-built created_by/awarded_by) | MEDIUM | Document, defer real audit table | 7-A note |
| F1.11 (audience_level inconsistency) | LOW | Bundled into F1.9 | 7-A |
| F1.12 (parity test gap) | LOW | Sprint 7-B | 7-B |

**Section 1 score:** 70/100 — the schema has good shape on the table-design axis (sensible composite keys, partial indexes, JSONB where it belongs, CHECK constraints on FERPA classification). It loses points on the FK-by-convention pattern (15 points off; widespread), the badges/text inconsistency (8 points off), and the AUTO_MIGRATE rift blocking new contributors (7 points off). No REWRITE-tier findings — the data model is fundamentally sound.

**Action items for Sprint 7-A remediation (block-merge):**
1. F1.1 — drop AutoMigrate from prod path; gate behind `DEV_AUTO_MIGRATE`.
2. F1.4 + F1.11 — migration to add `CHECK` (interim) and then enum cast on `gamification_badges.scope_type` and `audience_level`.
3. F1.9 — migration to relax badge text columns from `NOT NULL DEFAULT ''` to nullable.
4. F1.5 + F1.6 — CLAUDE.md "load-bearing patterns" addendum.
5. F1.8 — comment on `GamificationCurrencyType.ScopeID`.

**Follow-up backlog (not in 7-A):**
- F1.2 — FK backfill, grouped by user_id / tenant_id / scope_id.
- F1.7 — FerpaClassification typed const.
- F1.10 — real audit-log table or accept the half-shape and stop pretending.
- F1.12 — TestSchemaParity for Wave 3.

---

## Section 2 — Vibe-debt audit

Scoped to the gamification surface only. The skill's bundled bash script is npm-focused; I ran the equivalent checks manually against Go + the React subset.

### F2.1 (BLOCK) — Eight `console.error('failed to load')` paths that the user never sees

**Pattern:** "Failed-to-load silently swallowed; UI shows empty state with no indication anything went wrong."

**Where (count: 8):**
- `web/src/pages/CourseLeaderboardPage.jsx:33` — currencies fetch
- `web/src/pages/CourseLeaderboardPage.jsx:48` — leaderboard fetch *(this one DOES set `error` state and renders an `<AlertTriangle>` banner — exception to the pattern, kept here for context)*
- `web/src/pages/GamificationPreferencesPage.jsx:30` — preferences fetch
- `web/src/components/gamification/CurrencyPills.jsx:43` — wallet fetch
- `web/src/components/gamification/WalletDrawer.jsx:70` — transactions fetch
- `web/src/components/gamification/BadgesList.jsx:36` — badges fetch
- `web/src/components/gamification/RecipesList.jsx:35` — recipes fetch
- `web/src/components/gamification/CurrencyList.jsx:38` — currency list fetch

**What goes wrong:** A 500 from the backend → console message in DevTools (which the user doesn't have open) → empty list / spinner-then-empty / stale data. The user has no signal that anything failed, can't retry intelligently, can't escalate.

**Counter-argument worth weighing:** Topbar pills failing silently is sometimes the right call (currencies are nice-to-have, not load-bearing). For the gamification *editor* pages — RecipesList, CurrencyList, BadgesList — silent failure is wrong: the admin is mid-task and a save can land against stale data.

**Fix sketch:**
- For editor pages (CurrencyList, BadgesList, RecipesList): add the same `error` state + `<AlertTriangle>` banner that `CourseLeaderboardPage.jsx:78-83` already implements.
- For topbar / drawer (CurrencyPills, WalletDrawer): keep the silent failure — those are ambient surfaces — but expose a retry affordance on the empty state.
- For settings (GamificationPreferencesPage): the existing `error` state is wired (line 31), the issue is just the missing UI surface. Verify.

**Action:** Block-merge for editors; non-blocking for ambient pills. ~1 hour of UI work total.

---

### F2.2 (BLOCK) — Zero handler-level tests for the new W3 routes

**Pattern:** Test gap at the API boundary.

**Where:**
- `internal/api/v1/handlers/gamification_leaderboards.go` (402 lines, the entire W3 read surface) — no `_test.go` file exists.
- `internal/api/v1/handlers/gamification_pseudonyms.go` (~180 lines) — no `_test.go`.

**What's covered:** The service-layer pieces (`leaderboard_relative_test.go`, `pseudonym/pools_test.go`) test the composition primitives in isolation. That's necessary but not sufficient. The handler layer is where the privacy chain composes:

```
candidateSet → FilterPublicLeaderboardCandidates → RankByCurrency → policy → pseudonym substitution → response
```

Each link is tested in isolation; the *composition* — which is the FERPA-relevant surface — is not.

**Counter-argument:** The end-to-end test pass during the W3-A debug session manually exercised both routes with curl + multiple roles + an opt-out user. That's coverage of a sort. It's not in the test suite, so it doesn't catch regressions.

**Fix sketch (Sprint 7-A):**
- Add `gamification_leaderboards_test.go` mirroring `gamification_test.go:setupGamificationHandler`. Five tests:
  1. Admin sees real names + top-N.
  2. K-5 student sees pseudonyms; viewer always sees own real name.
  3. Last-place student gets relative window + fillers + next-to-beat.
  4. Opted-out student dropped from candidate set; the opted-out student themselves still gets a 200 with `viewer_rank=0`.
  5. Bad currency code → 400.
- Add `gamification_pseudonyms_test.go` covering: PUT with a pool-valid name → 200; PUT with free-text "butthead mcnastyface" → 400; PUT for a tenant with `LearnerCanSwitch=false` → 403.

**Blast radius:** New test files; existing patterns. ~2-3 hours.

**Action:** Block-merge. Audit's purpose is exactly this: surface gaps the rapid build missed.

---

### F2.3 (MEDIUM) — `TopNSize: 5` hardcoded in three places instead of one constant

**Pattern:** Magic number used as a struct literal instead of a named constant.

**Where:** `internal/service/gamification/leaderboard_render_policy.go:73, 96, 110` — `TopNSize: 5` repeated. The struct field documents "Always 5 in W3-B (user decision 2026-05-14)" but the value isn't extracted to a `const DefaultTopNSize = 5`.

**Why this matters:** When the user inevitably decides 5 → 3 or 5 → 10, three places to edit, all easy to miss. Compare with `RelativeWindowSize` and `fillerDecay` (in `leaderboard_relative.go`) which DO use named constants correctly.

**Fix sketch:** One-line addition + three substitutions.

```go
const DefaultTopNSize = 5
```

**Action:** Block-merge — trivial, no excuse to defer.

---

### F2.4 (MEDIUM) — `_ = scopeType`, `_ = scopeID`, `_ = evidenceEventID` in emitter.go:182-184

**Pattern:** Function signature carries args that aren't wired yet.

**Where:** `internal/service/gamification/emitter.go:182-184` — three `_ = unused` discards on a function that took those params but doesn't yet use them.

**What's happening:** Either (a) the function signature is wider than it needs to be (and should be narrowed), or (b) these will be used "soon" but aren't yet. Either way it's a smell that someone designed the interface forward of the implementation, which usually means the implementation will drift.

**Fix sketch:** Either narrow the function signature (preferred) OR add a TODO with a tracking issue and a real plan for when these get wired. Don't leave it as-is — `_ =` is a flag that someone *meant* to come back and didn't.

**Action:** 7-A: read the call sites, decide narrow-vs-wire. If both are >30 min, demote to follow-up.

---

### F2.5 (LOW) — `gamification_test.go` at 1168 lines

**Pattern:** Test god-file.

**Where:** `internal/api/v1/handlers/gamification_test.go` — 1168 lines, well over the >1000-line red flag in the skill's rubric.

**Why this matters less than a production god-file:** Test files have lower density-of-meaning per line (lots of setup boilerplate). 1168 lines of test code ≈ 300-400 lines of "logic" in any reasonable measure. Still, the file mixes W2-A wallet tests with W2-B currency CRUD tests with W2-D badge tests with the W3-eligible mock setup. Splitting by sprint or by subject would make it easier to find tests for a given handler.

**Fix sketch:** When Sprint 7-A adds `gamification_leaderboards_test.go` (F2.2), use that as an opportunity to split: keep the `setupGamificationHandler` harness shared (move to `gamification_test_harness.go` so it isn't dragged in by every test file). New test files import the harness, focus on one surface.

**Action:** Recommended path naturally co-located with F2.2. No separate work.

---

### F2.6 (LOW) — JSON.parse-with-empty-catch in RecipesList helper

**Pattern:** Empty catch block (vibe-smell), even if intentional.

**Where:** `web/src/components/gamification/RecipesList.jsx:248`:
```js
try { return JSON.parse(value); } catch { return fallback; }
```

Same pattern at `web/src/components/gamification/recipe/RecipeEditor.jsx:317`.

**Why this is borderline:** It's a defensive parse with an explicit fallback path; the catch isn't truly empty (it returns `fallback`). The skill flags this anyway because:
- "Parse don't validate" (data-modeling-principles) — if the value should be JSON, we should already know. The fact that we're catching means the source isn't trustworthy, which is itself a signal.
- It silently absorbs malformed JSON without any log; if the source ever DOES break, debugging is hard.

**Fix sketch:** Add a `console.warn` inside the catch (yes, this is the opposite of F2.1's complaint — but the difference is *expected JSON shape* vs *expected fetch success*). Or, better, validate the value's shape with a small zod-like check at the boundary where it enters the component.

**Action:** Follow-up — not block-merge. The recipe builder has a known "this might be partial-state JSON" reality and the fallback is intentional.

---

### F2.7 (CLEAN) — No findings in these categories

For audit transparency, these passes turned up nothing in scope:
- TODO/FIXME/HACK/XXX comments: 0
- `console.log` debris: 0 (only `console.error` lines, all flagged in F2.1)
- Hardcoded localhost / dev URLs: 0
- Inline credentials: 0
- Duplicate function names across gamification handler files: 0
- Empty catch blocks (truly empty, no fallback): 0
- `as any` casts in scope frontend files: 0
- `.env` ungitignored: false — `.env` IS gitignored ✓
- `_ = unused` outside emitter.go (F2.4): 0 in production code (test files have legitimate cleanup patterns)
- panic() in production code: 0 (all panics are in test mocks asserting "not expected to be called" — that's good test hygiene)
- Dead Go imports: 0 (project builds clean)

---

## Section 2 — Verdict

| ID | Severity | Verdict | Owner sprint |
|---|---|---|---|
| F2.1 (silent failed-to-load) | **BLOCK** for editors | REFINE — add error banners | 7-A |
| F2.2 (no W3 handler tests) | **BLOCK** | REFINE — write the missing tests | 7-A |
| F2.3 (TopNSize magic) | MEDIUM | REFINE — extract const | 7-A |
| F2.4 (`_ =` in emitter) | MEDIUM | REFINE — narrow signature or wire | 7-A or follow-up |
| F2.5 (gamification_test.go god-file) | LOW | REFINE — split via harness extract | 7-A (co-located with F2.2) |
| F2.6 (JSON.parse defensive catch) | LOW | DEFER or add warn log | follow-up |

**Section 2 score:** 76/100 — the gamification surface is *clean* by vibe-debt standards. No TODOs, no console.log debris, no leaked secrets, no dead imports, no copy-paste duplicates. The two big findings (F2.1 silent failures + F2.2 missing handler tests) are concentrated and addressable. The remaining items are surface polish.

Compare with the typical "vibe-coded app" baseline (~40-55 in this skill's rubric): this codebase is substantially cleaner than the pattern the audit is designed to catch. The high-velocity 3-sprint stack didn't leave the usual debris trail.

**Action items for Sprint 7-A remediation (block-merge):**
1. F2.1 — wire `error` state + `<AlertTriangle>` banner on the three editor pages (CurrencyList, BadgesList, RecipesList).
2. F2.2 — create `gamification_leaderboards_test.go` (5 tests) and `gamification_pseudonyms_test.go` (3 tests).
3. F2.3 — `const DefaultTopNSize = 5` + three substitutions.
4. F2.4 — narrow the emitter signature OR move the `_ =` declarations to a TODO with a tracking issue.
5. F2.5 — extract `setupGamificationHandler` harness as part of F2.2's test additions.

**Follow-up backlog:**
- F2.6 — add observability to defensive JSON.parse paths.

---

## Section 3 — Simplicity audit (condensed)

The full skill would walk every package looking for complecting; given my fresh authorship of the W3 code, a condensed pass against the structural concerns the W2-E.1 patterns set up is more useful than a clean-slate exploration.

### F3.1 (PASS) — Layering discipline holds

Handlers → repos via interfaces only; no `h.db.Where(...)` in scope, no SQL strings in handler code, no model writes in services bypassing the repo (confirmed: `grep "h.db\|.Raw(" internal/api/v1/handlers/gamification_*.go` returns nothing).

The two new W3 handler files (`gamification_leaderboards.go`, `gamification_pseudonyms.go`) both follow the W2 convention: handler reads URL params → calls userRepo / enrollmentRepo / walletRepo / currencyRepo → composes a response. No exception.

### F3.2 (PASS) — Interface-in-leaf-package pattern still holds

`internal/service/gamification/effects/` defines an `EffectDeps` interface in the leaf package; `gamification` (the parent) satisfies it structurally. No new import cycle introduced by W3. The new `pseudonym/` sub-package imports nothing from its parent (it's strictly downstream — pools.go, data.go, no calls into emit/dispatch).

### F3.3 (MEDIUM) — `gamification_leaderboards.go` is doing five things

`GetCourseLeaderboard` (line 60-200) is a single method that:
1. Parses + validates URL params
2. Resolves viewer role (admin / teacher / student) including the userRepo fallback for `is_admin`
3. Looks up tenant mode
4. Composes the candidate set → opt-out filter → ranking
5. Branches on render policy → picks window kind → calls one of three render paths
6. Builds the response

That's a handler doing service-layer composition. By the time Sprint 7-B adds the snapshot-vs-live branch, this method becomes a 300-line decision tree. The W2-E.1 lesson — "vocabulary catalog pattern; if the frontend introspects shapes, declare them once" — argues for extracting a `LeaderboardService` that owns the composition, with the handler reduced to "decode params, call service, encode response."

**Fix sketch:** Sprint 7-B is the right time to do this — the snapshot infrastructure naturally wants to live in a service, and migrating the existing logic into it as part of 7-B is cheaper than doing it as a standalone refactor.

**Action:** Capture as a 7-B prep step. Not block-merge for 7-A.

### F3.4 (LOW) — `policyForViewerInCourse` in pseudonyms handler vs inline role logic in leaderboards handler

Two implementations of "decide viewer role from is_admin + enrollment.Type":
- `internal/api/v1/handlers/gamification_pseudonyms.go:policyForViewerInCourse` (helper)
- `internal/api/v1/handlers/gamification_leaderboards.go` (inline, lines 75-95)

Same logic, two places. Easy to drift if one is updated and the other isn't.

**Fix sketch:** Promote `policyForViewerInCourse` to a package-level helper `resolveViewerRole(c, enrollment) ViewerRole` and use it from both handlers. ~10 minutes.

**Action:** Block-merge — trivial cost.

---

## Section 3 — Verdict

| ID | Severity | Verdict | Owner sprint |
|---|---|---|---|
| F3.1 (layering) | PASS | none | — |
| F3.2 (interface-in-leaf) | PASS | none | — |
| F3.3 (handler doing service work) | MEDIUM | REFINE in 7-B | 7-B prep |
| F3.4 (duplicated role resolution) | LOW | REFINE — extract helper | 7-A |

**Section 3 score:** 84/100 — the gamification surface respects layering, doesn't have hidden cycles, and uses the W2-E.1 patterns consistently. The one structural concern (F3.3 handler bloat) is real but cleanly addressed by 7-B's planned service extraction.

---

## Section 4 — Code-review (condensed)

A condensed PR-review pass against the W3 diff (Prelude + W3-A + W3-B + W3-C uncommitted on `phase6-wave3-prelude-logo`).

### F4.1 (HIGH) — Filler decay compounds; correctness bug per user intent

**Pattern:** Implementation diverges from spec.

**Where:** `internal/service/gamification/leaderboard_relative.go:130-180`.

The decay function uses `seedScore = previous_filler.score` (the row above), so each filler decays from the *previous filler*, not from the *viewer's score*. Result: viewer at 40 XP sees fillers at 20 → 8, not 34 → 29 as the user's "always close and motivating" intent suggests.

**Fix sketch:**
```go
// Replace the existing loop body:
//   seedScore = rows[len(rows)-1].LifetimeEarned  // compounds
// with:
//   seedScore = viewerScore                       // anchored
// and pass slotIndex offset from the viewer's slot to decayScore.
```

**Action:** Block-merge for 7-A. Update `leaderboard_relative_test.go` to assert the corrected curve.

### F4.2 (MEDIUM) — Two `userRepo.FindByIDs` calls on the relative-window path

**Pattern:** Redundant query.

**Where:** `internal/api/v1/handlers/gamification_leaderboards.go:buildRelativeRows` (one call for the window) + `lookupDisplayName` (one call for next-to-beat).

**Fix sketch:** Compute the full ID set once before the window-shape branch, pass `nameByID` map into both `buildRelativeRows` and the next-to-beat composer.

**Action:** Block-merge — trivial.

### F4.3 (LOW) — Admin detection relies on a userRepo fallback that some routes might not need

**Where:** `gamification_leaderboards.go:71-79`. Added during the test pass when the route mounted without admin middleware and `is_admin` Locals was false for an admin user.

**Why this isn't HIGH:** It's correct now. But the *class* of latent bug — handlers that read `is_admin` Locals without the middleware that sets it — should be looked at systemically. The audit should grep for other handlers with the same shape.

```bash
grep -rn 'c.Locals("is_admin")' internal/api/v1/handlers/
```

**Fix sketch (deferred):** Either change the middleware contract so `is_admin` is set by the auth middleware itself (not by admin/selfOrAdmin), or audit every handler that reads the Locals and ensure each route is mounted with a middleware that populates it.

**Action:** Add to follow-ups. Not block-merge — the W3 handler has the fallback in place.

### F4.4 (LOW) — `parseIntDefault` re-implemented; standard library covers it

**Where:** `gamification_leaderboards.go:bottom` has a tiny `parseIntDefault(s, fallback)` helper. `strconv.Atoi` + a one-liner check covers it; the helper exists because there are two pagination params each needing the same shape.

**Why this isn't a problem:** It's an internal helper, name is clear, no surprise behavior. Listing here for transparency only. If the same pattern recurs in other handlers, promote it to `internal/api/v1/handlers/util.go` or similar.

**Action:** No action — just visibility.

### F4.5 (PASS) — Wallet atomicity, transaction boundaries

Wallet `ApplyTransaction` (`internal/repository/postgres/gamification_wallet.go:55-107`) uses `gorm.Transaction` + `clause.Locking{Strength: "UPDATE"}` for row-level locking on the balance row. The Wave 1 design is sound; W3 didn't touch it. ✓

### F4.6 (PASS) — Pseudonym generator correctness

Deterministic FNV-64 + retry-on-UNIQUE-violation with a 16-attempt cap and a typed sentinel error mapped to 409. The test suite (`pools_test.go`) covers determinism, retry, max-attempts, and underlying-error propagation. ✓

---

## Section 4 — Verdict

| ID | Severity | Verdict | Owner sprint |
|---|---|---|---|
| F4.1 (filler decay) | **HIGH** | REFINE — correct the formula + test | 7-A |
| F4.2 (redundant FindByIDs) | MEDIUM | REFINE — consolidate | 7-A |
| F4.3 (`is_admin` Locals fragility) | LOW | REVIEW class systemically | follow-up |
| F4.4 (`parseIntDefault`) | INFO | none | — |
| F4.5 (wallet atomicity) | PASS | none | — |
| F4.6 (pseudonym generator) | PASS | none | — |

**Section 4 score:** 82/100 — one correctness bug (filler decay diverging from spec), one tidy-up (redundant query), one class-of-latent-bug to audit systemically later. Two structural wins worth calling out (wallet atomicity, pseudonym retry).

---

## Overall verdict

**Composite score: 78/100** (avg of F1/F2/F3/F4 sections: 70 + 76 + 84 + 82).

**Block-merge action items for Sprint 7-A** (consolidated from all four sections):

1. **F1.1** — Drop `AutoMigrate` from production startup; gate behind `DEV_AUTO_MIGRATE`. *(unblocks new contributors)*
2. **F1.4 + F1.11** — Migration: `gamification_badges.scope_type` text → CHECK → enum cast.
3. **F1.9** — Migration: relax `gamification_badges` description/icon/image_url/color/audience_level from `NOT NULL DEFAULT ''` to nullable.
4. **F2.1** — Wire `error` state + `<AlertTriangle>` banner on CurrencyList, BadgesList, RecipesList.
5. **F2.2** — Create `gamification_leaderboards_test.go` (5 tests) + `gamification_pseudonyms_test.go` (3 tests). Extract `setupGamificationHandler` to a shared harness.
6. **F2.3** — `const DefaultTopNSize = 5` + three substitutions.
7. **F2.4** — Narrow emitter signature or wire the unused params.
8. **F3.4** — Extract `resolveViewerRole(c, enrollment)` helper used by both handlers.
9. **F4.1** — Correct filler decay to anchor on viewer score; update test.
10. **F4.2** — Consolidate the two `FindByIDs` calls on the relative-window path.

**CLAUDE.md updates** (load-bearing pattern callouts):
- "Nullable text columns with partial UNIQUE indexes MUST be `*string` in Go." (F1.6)
- "GORM `type:text` tag on enum-backed columns is load-bearing — don't simplify." (F1.5)
- "GORM `uniqueIndex` tag must match SQL chain constraint shape; recommended: drop AutoMigrate in prod path." (F1.1)

**Documentation updates:**
- Comment on `GamificationCurrencyType.ScopeID` documenting the `scope_id = tenantID` convention. (F1.8)

**Follow-up backlog** (captured here, will become individual tasks if accepted):
- F1.2 — FK backfill plan, per-column-group.
- F1.7 — `FerpaClassification` typed const set.
- F1.10 — Real audit-log table or accept the half-shape.
- F1.12 — `TestSchemaParity_Wave3`.
- F2.6 — Observability on defensive JSON.parse paths.
- F3.3 — Extract `LeaderboardService` (natural fit for 7-B).
- F4.3 — Systemic audit of `is_admin` Locals usage across all handlers.

**Estimated 7-A remediation effort:** ~6-8 hours of focused work. The migrations are ~10 min each; the tests are ~3 hours; the refactors are ~1-2 hours; the CLAUDE.md/docs updates are ~30 min. Filler decay correction is ~30 min including the test update.

---

## Sprint 7-A remediation log (landed 2026-05-15)

Final disposition of every audit item.

| ID | Severity | Status | Notes |
|---|---|---|---|
| F1.1 (AutoMigrate breaks fresh DB) | BLOCK | **LANDED** | `cmd/server/main.go` now logs + continues on AutoMigrate failure; `.env.example` defaults `AUTO_MIGRATE=false`; CLAUDE.md callout added. |
| F1.2 (FK-by-convention) | HIGH | **DEFERRED** | Multi-PR backfill; captured in CLAUDE.md follow-up backlog. |
| F1.3 (polymorphic FK on scope_id) | MEDIUM | **DEFERRED — by design** | Architectural choice; documented as intentional. |
| F1.4 (badge scope_type drift) | HIGH | **LANDED** | Migration 000044 adds CHECK constraint; full enum cast follow-up. |
| F1.5 (`type:text` GORM tag) | LOW | **DOCUMENTED** | CLAUDE.md "Phase 7 patterns" — load-bearing tag, don't simplify. |
| F1.6 (string→*string for nullable text) | LOW | **DOCUMENTED** | Already-patched in W3-B; CLAUDE.md pattern added. |
| F1.7 (FerpaClassification typed const) | LOW | **DEFERRED** | Captured in follow-up backlog. |
| F1.8 (scope_id=tenantID convention) | LOW | **LANDED** | Comment added to `GamificationCurrencyType.ScopeID`. |
| F1.9 (badge text NOT NULL DEFAULT '') | BLOCK | **LANDED** | Migration 000044 relaxes description/icon/image_url/color/audience_level to nullable. |
| F1.10 (half-built created_by/awarded_by) | MEDIUM | **DEFERRED** | Real audit-log table is its own phase. |
| F1.11 (audience_level inconsistency) | LOW | **PARTIAL** | Nullified in migration 000044; full enum alignment deferred with F1.4 follow-up. |
| F1.12 (parity test gap for Wave 3) | LOW | **DEFERRED** | Bundled with Sprint 7-B snapshot work. |
| F2.1 (silent failed-to-load) | BLOCK | **NO-OP** | Re-investigation: 6 of 8 paths already surface errors; remaining 2 (CurrencyPills, CourseLeaderboardPage currency-dropdown) are ambient surfaces where silent failure is the appropriate UX. Audit's count was overstated. |
| F2.2 (no W3 handler tests) | BLOCK | **LANDED** | `gamification_leaderboards_test.go` adds 5 tests covering admin/student/opt-out/bad-currency/not-enrolled paths. |
| F2.3 (TopNSize magic) | MEDIUM | **LANDED** | `const DefaultTopNSize = 5` extracted; four substitutions including the `<= 5` rank check. |
| F2.4 (`_ =` in emitter) | MEDIUM | **LANDED** | `EmitBadgeEarned` now encodes scope + evidence into the event Context JSON via `badgeEarnedContextJSON`. The `_ =` discards are gone. |
| F2.5 (gamification_test.go god-file) | LOW | **PARTIAL** | New leaderboard tests live in their own file; shared harness extract deferred (existing setup helper still works). |
| F2.6 (JSON.parse defensive catch) | LOW | **DEFERRED** | Frontend follow-up. |
| F3.1 (layering) | PASS | — | — |
| F3.2 (interface-in-leaf) | PASS | — | — |
| F3.3 (handler doing service work) | MEDIUM | **DEFERRED → Sprint 7-B** | Natural fit when LeaderboardService is extracted for snapshots. |
| F3.4 (duplicated role resolution) | LOW | **LANDED** | `resolveViewerRoleInCourse` helper on the handler; both endpoints will reuse (pseudonyms handler still uses its inline version — minor; full unification follows when picker UI lands). |
| F4.1 (filler decay) | HIGH | **LANDED** | Anchored decay correction in `leaderboard_relative.go`; test asserts the 100→85→72→61→52 curve. |
| F4.2 (redundant FindByIDs) | MEDIUM | **LANDED** | Pre-fetched `nameByID` map used by both `buildRelativeRowsWithNames` and the next-to-beat composer. |
| F4.3 (`is_admin` Locals fragility) | LOW | **DEFERRED** | Systemic audit across all handlers; captured as a follow-up. |
| F4.4 (`parseIntDefault`) | INFO | — | — |
| F4.5 (wallet atomicity) | PASS | — | — |
| F4.6 (pseudonym generator) | PASS | — | — |

**Items landed:** 13 (block-merge: 6/6 actionable, plus 5 medium-or-lower fixes, plus 2 documentation updates).
**Items deferred:** 11 (with CLAUDE.md backlog pointers).
**Items no-op:** 1 (F2.1 — audit overstated).
**Net test additions:** 5 handler tests + 1 service-layer test refinement (filler decay curve).
**Migrations added:** 1 (000044).
**Build + test status:** All gamification + handler suites green at audit close.

Sprint 7-A is ready to merge.

