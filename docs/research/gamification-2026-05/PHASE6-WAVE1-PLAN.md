# Phase 6 / Wave 1 Plan — Gamification Foundations

*2026-05-12. The concrete implementation plan for Wave 1 of [SYNTHESIS.md](./SYNTHESIS.md). Maps onto Paper LMS's existing Go + Fiber + GORM + Postgres layout.*

**Revision history:**
- 2026-05-12 v1 — initial plan
- 2026-05-12 v2 — dropped Canvas Live Events bridge (Paper LMS *is* the LMS, not a Canvas LTI tool); replaced fixed currency enum with user-defined `gamification_currency_types` table (MyCred pattern); confirmed both Khan spaced-retrieval and decaying-average mastery methods ship side-by-side, selectable per outcome.

---

## Goal

Land the schema + Go scaffolding that every later wave consumes. **No teacher-facing UI, no badge issuance, no leaderboards** in Wave 1 — those are Waves 2–3. Wave 1 is plumbing.

Concretely:
- The `events` table (xAPI-shaped) and its emitter API.
- The `rules` + `rule_evaluations` tables and the predicate evaluator.
- The **user-defined currency wallet** — `currency_types` + `wallet_balances` + `wallet_transactions`. Default seed: `xp`, `gems`, `mastery_points`, `reputation`. Teachers and admins can define unlimited additional currencies (`coins`, `class_bucks`, anything) scoped to site/course/section.
- Tenant mode flag + FERPA field-tag enforcement at the data-access layer.
- xAPI-shape statement emission for the first 20 core triggers (Learning Progress + Assessment Mastery), all firing from internal services — no external bridge needed.

---

## Decisions resolved (2026-05-12)

1. ~~Canvas Live Events transport~~ — **REMOVED.** Paper LMS replaces Canvas; it does not run alongside Canvas. Events come from internal services calling `gamification.Emit()` directly. The Canvas bridge would only matter for a future migration-import tool (Phase X) or an edge-case hybrid deploy, neither of which is Wave 1.
2. **Mastery calc methods** — **ship all six**, configurable per outcome (or override per-rule via the `OutcomeMastery` predicate's `calc_method` parameter). Defaults:
   - `khan_spaced_retrieval` — practice/skill-based formative outcomes (math facts, vocab, procedural skills)
   - `decaying_average` (default weight = 0.65 recent / 0.35 prior) — rubric-graded summative outcomes (essays, projects)
   - `most_recent` — one-shot certifications
   - `highest` — best-effort credentials
   - `n_times` — skill-checks requiring repeated demonstration (default n=3)
   - `weighted_average` — explicit teacher-weighted multi-source mastery
3. **Currency model** — **fully user-defined, MyCred-style.** A `gamification_currency_types` table replaces the fixed enum. Each tenant/course/section can define any number of currencies with custom names, icons, colors, decay rules, spendability, FERPA classification, and topbar visibility. Four currencies are **system-seeded** on tenant creation (`xp`, `gems`, `mastery_points`, `reputation`) — these can be renamed but not deleted because rules and capabilities reference them by code.

Remaining non-blocking watch items (don't block Wave 1):
- Effort-derived metrics on public leaderboards: legal review. *Blocks Wave 3.*
- USPTO FTO search (Rasch/IRT). *Blocks Wave 4.*
- COPPA 2.0 status. *Tracked at every wave boundary.*

---

## Migration plan

Next migration number: **000032**. Following the project's two-migration rename convention (CONTRIBUTING.md) where applicable; Wave 1 is mostly additive so single-up/down is fine.

### `000032_gamification_event_bus.up.sql` — event store

```sql
CREATE TABLE gamification_events (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  occurred_at   TIMESTAMPTZ NOT NULL,
  emitted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  tenant_id     UUID        NOT NULL,
  actor_id      UUID        NOT NULL,             -- internal opaque user id
  verb          TEXT        NOT NULL,             -- 'completed' | 'mastered' | 'answered' | ...
  object_type   TEXT        NOT NULL,             -- 'Assignment' | 'Quiz' | 'Outcome' | ...
  object_id     UUID,
  result        JSONB,                            -- {score, success, completion, response, duration_ms}
  context       JSONB,                            -- {course_id, section_id, registration, source_event_id}
  source        TEXT        NOT NULL,             -- 'internal' | 'lti' | 'webhook' | 'migration_import'
  source_event_id TEXT,                           -- idempotency key from external systems
  policy_flags  TEXT[]      NOT NULL DEFAULT '{}', -- ['ferpa_protected','education_record',...]
  signature     TEXT                              -- for webhook re-emission
);

CREATE INDEX idx_gam_events_actor_occurred ON gamification_events (actor_id, occurred_at DESC);
CREATE INDEX idx_gam_events_verb_object    ON gamification_events (verb, object_type, object_id);
CREATE INDEX idx_gam_events_tenant_time    ON gamification_events (tenant_id, occurred_at DESC);
CREATE UNIQUE INDEX uniq_gam_events_source_event_id
  ON gamification_events (source, source_event_id)
  WHERE source_event_id IS NOT NULL;
```

### `000033_gamification_rules.up.sql` — rules engine

```sql
CREATE TYPE gamification_scope_type AS ENUM ('site','district','school','course','section');
CREATE TYPE gamification_audience   AS ENUM ('k5','m68','h912','higher_ed','corp','pro');

CREATE TABLE gamification_rules (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       UUID NOT NULL,
  scope_type      gamification_scope_type NOT NULL,
  scope_id        UUID NOT NULL,
  audience_level  gamification_audience NOT NULL,
  name            TEXT NOT NULL,
  description     TEXT,
  enabled         BOOLEAN NOT NULL DEFAULT TRUE,
  trigger_event   JSONB NOT NULL,    -- {kind:'OnEvent', verb:'completed', object_type:'Quiz'} | OnSchedule | OnManualTrigger
  condition_set   JSONB NOT NULL,    -- recursive predicate tree
  effects         JSONB NOT NULL,    -- ordered list of effect specs
  cooldown_seconds  INT,
  max_per_window  JSONB,              -- {window:'day'|'week'|'lifetime', count:N}
  created_by      UUID,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gam_rules_scope ON gamification_rules (scope_type, scope_id) WHERE enabled;
CREATE INDEX idx_gam_rules_tenant ON gamification_rules (tenant_id) WHERE enabled;

CREATE TABLE gamification_rule_evaluations (
  rule_id          UUID NOT NULL REFERENCES gamification_rules(id) ON DELETE CASCADE,
  user_id          UUID NOT NULL,
  evaluated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  predicate_state  JSONB,
  result           BOOLEAN NOT NULL,
  effects_fired    JSONB,
  triggering_event_id UUID REFERENCES gamification_events(id) ON DELETE SET NULL,
  PRIMARY KEY (rule_id, user_id, evaluated_at)
);

CREATE INDEX idx_gam_eval_user_rule_time
  ON gamification_rule_evaluations (user_id, rule_id, evaluated_at DESC);
```

### `000034_gamification_currencies_and_wallet.up.sql` — user-defined currencies + wallet

```sql
-- User-defined currency types. MyCred pattern. Each tenant/course/section can define unlimited currencies.
CREATE TABLE gamification_currency_types (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id            UUID NOT NULL,
  scope_type           gamification_scope_type NOT NULL,    -- 'site' | 'course' | 'section' (rare: 'district'|'school')
  scope_id             UUID NOT NULL,
  code                 TEXT NOT NULL,                        -- 'xp' | 'coins' | 'gems' | 'class_bucks' | ...
  display_label        TEXT NOT NULL,                        -- "XP", "Coins", "Gems", "Class Bucks"
  display_label_plural TEXT,                                 -- optional; "Energy" might have plural "Energies"; fallback to display_label
  icon                 TEXT,                                 -- lucide-react icon name or asset URL
  color                TEXT,                                 -- hex; drives the topbar pill
  display_order        INT NOT NULL DEFAULT 0,              -- left-to-right in topbar
  spendable            BOOLEAN NOT NULL DEFAULT FALSE,       -- can balance decrease through purchases?
  monotonic            BOOLEAN NOT NULL DEFAULT TRUE,        -- if true, balance never decreases regardless of spendable
  ferpa_classification TEXT NOT NULL DEFAULT 'non_PII' CHECK (ferpa_classification IN
    ('directory_information','education_record','non_PII','instructor_metadata')),
  max_balance          BIGINT,                               -- NULL = uncapped
  decay_policy         JSONB,                                -- {kind:'none'|'inactivity'|'expire_after_days', ...}
  visible_to_student   BOOLEAN NOT NULL DEFAULT TRUE,
  visible_in_topbar    BOOLEAN NOT NULL DEFAULT TRUE,
  system_owned         BOOLEAN NOT NULL DEFAULT FALSE,       -- if true, currency cannot be deleted (xp/gems/mastery_points/reputation)
  description          TEXT,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, scope_type, scope_id, code)
);

CREATE INDEX idx_gam_currency_tenant_scope
  ON gamification_currency_types (tenant_id, scope_type, scope_id);
CREATE INDEX idx_gam_currency_topbar
  ON gamification_currency_types (tenant_id, visible_in_topbar, display_order)
  WHERE visible_in_topbar;

CREATE TABLE gamification_wallet_balances (
  user_id          UUID NOT NULL,
  currency_type_id UUID NOT NULL REFERENCES gamification_currency_types(id) ON DELETE RESTRICT,
  balance          BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
  lifetime_earned  BIGINT NOT NULL DEFAULT 0,       -- monotonic; used for leaderboards even if balance is spendable
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, currency_type_id)
);

CREATE TABLE gamification_wallet_transactions (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             UUID NOT NULL,
  currency_type_id    UUID NOT NULL REFERENCES gamification_currency_types(id) ON DELETE RESTRICT,
  delta               BIGINT NOT NULL,             -- positive = earn, negative = spend
  reason              TEXT NOT NULL,                -- 'rule:<rule_id>' | 'manual:<actor_id>' | 'spend:<sku>'
  triggering_event_id UUID REFERENCES gamification_events(id) ON DELETE SET NULL,
  triggering_rule_id  UUID REFERENCES gamification_rules(id) ON DELETE SET NULL,
  policy_flags        TEXT[] NOT NULL DEFAULT '{}',
  occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gam_wallet_tx_user_time
  ON gamification_wallet_transactions (user_id, occurred_at DESC);
CREATE INDEX idx_gam_wallet_tx_rule
  ON gamification_wallet_transactions (triggering_rule_id)
  WHERE triggering_rule_id IS NOT NULL;
```

**Note on `currency_type_id` scoping**: balance rows are *not* further scoped by course/section in the schema — that scoping lives in `currency_type_id` itself (each course-scoped currency has its own UUID). A learner enrolled in two sections of a course with section-scoped currencies will have two balance rows. This keeps queries simple and indexes effective.

### `000035_tenant_mode_and_ferpa_tags.up.sql` — compliance plumbing

```sql
ALTER TABLE accounts
  ADD COLUMN tenant_mode gamification_audience NOT NULL DEFAULT 'higher_ed',
  ADD COLUMN coppa_strict BOOLEAN NOT NULL DEFAULT FALSE;

-- FERPA field tags: a lookup of which JSONB result/context paths are education_record
-- vs directory_information vs non_PII. Drives the data-access enforcement layer.
CREATE TABLE gamification_ferpa_field_tags (
  object_type    TEXT NOT NULL,
  field_path     TEXT NOT NULL,                 -- JSONPath into events.result/context
  classification TEXT NOT NULL CHECK (classification IN
    ('directory_information','education_record','non_PII','instructor_metadata')),
  PRIMARY KEY (object_type, field_path)
);
```

### Currency seeding — Go-side, not SQL

The four system-seeded currencies (`xp`, `gems`, `mastery_points`, `reputation`) are inserted by a Go-side seeder that runs on tenant creation (and a backfill one-shot for existing tenants on first deploy). Reasoning: the seeder needs to call into the `accounts` repository to enumerate tenants, and locales/display-label translations are easier from Go than from SQL.

Seeded shape (site-scoped per tenant):

```go
// internal/service/gamification/seed.go
var systemCurrencies = []CurrencyTypeSeed{
  {
    Code: "xp", Label: "XP", LabelPlural: "XP",
    Icon: "zap", Color: "#F59E0B", Order: 1,
    Spendable: false, Monotonic: true,
    Ferpa: "non_PII", VisibleInTopbar: true, SystemOwned: true,
    Description: "Experience points. Earned through any productive activity. Never decreases.",
  },
  {
    Code: "gems", Label: "Gem", LabelPlural: "Gems",
    Icon: "gem", Color: "#A855F7", Order: 2,
    Spendable: true, Monotonic: false,
    Ferpa: "non_PII", VisibleInTopbar: true, SystemOwned: true,
    Description: "Rare currency for the shop. Earned through quests, perfect scores, and surprises.",
  },
  {
    Code: "mastery_points", Label: "Mastery Point", LabelPlural: "Mastery Points",
    Icon: "target", Color: "#0EA5E9", Order: 3,
    Spendable: false, Monotonic: true,
    Ferpa: "education_record", VisibleInTopbar: false, SystemOwned: true,
    Description: "Skill mastery. Tied to learning outcomes. Visible to student and teacher only.",
  },
  {
    Code: "reputation", Label: "Rep", LabelPlural: "Rep",
    Icon: "shield-check", Color: "#10B981", Order: 4,
    Spendable: false, Monotonic: true,
    Ferpa: "non_PII", VisibleInTopbar: true, SystemOwned: true,
    Description: "Community reputation. Earned through helpful contributions. Unlocks capabilities.",
  },
}
```

`system_owned = TRUE` rows cannot be deleted by tenant admins; they can only be renamed (`display_label`, `icon`, `color`) or have their topbar visibility toggled. The `code` is locked because rules reference currencies by code.

**Down migrations**: standard reverse-order drops. Currencies must be dropped after wallet tables. Be careful with the enums — drop them last.

**Schema parity**: every new model must land in `internal/domain/models/` *in the same PR* as the migration, per CLAUDE.md's `TestSchemaParity_Wave1` rule. Backfill paths for existing `accounts` rows: the `tenant_mode` default lands as `higher_ed`; admins migrate K-12 tenants manually (or via a one-shot data migration in Wave 2). The currency seeder runs as a one-shot in `cmd/migrate` after migrations apply.

---

## Go module layout

New code lands under existing packages — no new top-level dirs. **No `integrations/canvas/` directory** (Paper LMS replaces Canvas; no inbound bridge).

```
internal/
  domain/models/
    gamification_event.go                 # Event struct
    gamification_rule.go                  # Rule, RuleEvaluation
    gamification_currency_type.go         # CurrencyType
    gamification_wallet.go                # WalletBalance, WalletTransaction
    gamification_ferpa_tag.go             # FerpaFieldTag
  repository/
    interfaces.go                         # +GamificationEventRepository
                                          # +GamificationRuleRepository
                                          # +GamificationCurrencyTypeRepository
                                          # +WalletRepository
                                          # +FerpaTagRepository
    postgres/
      gamification_event.go
      gamification_rule.go
      gamification_currency_type.go
      gamification_wallet.go
      gamification_ferpa_tag.go
  service/
    gamification/
      emitter.go                          # public API: Emit(ctx, event) — the single entry point
      bus.go                              # internal: dispatches to rule engine
      seed.go                             # system currency seeder (tenant creation + backfill)
      predicates/                         # one file per atomic predicate type
        submitted_assignment.go
        submitted_quiz.go
        viewed_content.go
        outcome_mastery.go                # supports all 6 calc_methods
        khan_mastery_level.go             # convenience wrapper around OutcomeMastery with calc_method='khan_spaced_retrieval'
        xp_threshold.go                   # generalizes to CurrencyThreshold(code, amount)
        currency_threshold.go             # explicit predicate; references currency by code
        earned_badge.go
        reputation_threshold.go
        condition_set.go                  # AND / OR / N_OF_M composition
        ...
      effects/                            # one file per effect type
        award_currency.go                 # references currency_type_id (resolved from code at apply time)
        notify.go                         # Wave 1 stub; full impl in Wave 2
        ...
      mastery/                            # calc_method implementations
        khan_spaced_retrieval.go
        decaying_average.go
        most_recent.go
        highest.go
        n_times.go
        weighted_average.go
      evaluator.go                        # given a rule + event + actor state, returns Effect[]
      ferpa_guard.go                      # enforces field-tag classification on emit
  api/v1/
    handlers/
      gamification_event.go               # POST /api/v1/gamification/events (internal testing hook)
      gamification_currency.go            # GET /api/v1/gamification/currencies (list types for a scope)
      gamification_wallet.go              # GET /api/v1/users/:id/wallet (read-only in Wave 1)
```

**Naming convention**: prefix tables and packages with `gamification_` / `gamification/` to make it obvious which domain owns the schema. Models drop the prefix internally (`models.Event` inside the `gamification` package isn't ambiguous).

---

## Predicate evaluator design

The evaluator is the heart of Wave 1. Build it as **pure functions over a snapshot of actor state**, not as stateful evaluation. This lets us:
- Re-evaluate rules on backfill (e.g., when a new rule is created, run it once over each user's history).
- Test predicates in isolation without spinning up a DB.
- Cache actor-state snapshots when many rules listen to the same event.

```go
// internal/service/gamification/predicates/predicate.go
type Predicate interface {
    Evaluate(ctx context.Context, actor ActorSnapshot) (bool, Trace)
}

type ActorSnapshot struct {
    UserID         uuid.UUID
    TenantID       uuid.UUID
    Now            time.Time
    Submissions    map[uuid.UUID]SubmissionState  // assignment_id -> latest
    QuizAttempts   map[uuid.UUID]QuizState
    OutcomeMastery map[uuid.UUID]MasteryState     // outcome_id -> calc'd level (using the outcome's configured calc_method, overridable per predicate)
    WalletBalances map[uuid.UUID]int64            // currency_type_id -> balance
    CurrencyByCode map[string]uuid.UUID           // resolve 'xp' -> currency_type_id at evaluation time
    EarnedBadges   []uuid.UUID
    Enrollments    []EnrollmentState
    LastLogin      time.Time
    // ... loaded lazily per predicate type
}

type ConditionSet struct {
    Op        Op  // AND, OR, N_OF_M
    Threshold int // for N_OF_M
    Children  []Predicate
}

func (c ConditionSet) Evaluate(ctx context.Context, a ActorSnapshot) (bool, Trace) {
    // ... canonical short-circuit AND/OR; N-of-M counts true results
}
```

**Mastery calc_method**: the `OutcomeMastery` predicate accepts an optional `calc_method` override in its JSONB spec. When absent, falls back to the outcome's configured default method. The `service/gamification/mastery/` package contains one impl per method; each takes `(events []Event, params Params) MasteryState` and is pure.

```go
// Example predicate spec in condition_set JSONB
{
  "kind": "OutcomeMastery",
  "outcome_id": "uuid",
  "level": "proficient",
  "calc_method": "khan_spaced_retrieval"   // optional; default = outcome's configured method
}
```

**Currency references by code, not UUID**: rules are authored against currency *codes* (`"xp"`, `"coins"`) so they survive currency renames and are portable across tenants. The evaluator resolves code → currency_type_id at apply time via `ActorSnapshot.CurrencyByCode`. This matters because a teacher creating a class shop with their own `coins` currency can copy a rule template and have it Just Work.

**Loading ActorSnapshot**: lazy — each predicate declares what slice of state it needs; the evaluator unions those before loading. This keeps the snapshot cheap when only a few predicates fire on a given event.

**Trace**: every evaluation returns a structured trace stored in `predicate_state` for debuggability. Critical for teachers asking "why didn't this rule fire?"

---

## Wave 1 task breakdown

14 sub-tasks, roughly 4–6 weeks at full focus. Parallel-agent isolated worktrees where independent. (Task 11 — Canvas Live Events bridge — removed; net task count down by one but rebalanced.)

| # | Task | Depends on | Worktree-safe |
|---|---|---|---|
| 1 | ~~Settle blocking decisions~~ — done | — | n/a |
| 2 | Write migrations 000032–000035 | — | yes |
| 3 | Add domain models + GORM tags (5 models incl. CurrencyType) | 2 | yes (same worktree as 2) |
| 4 | Repository interfaces + Postgres impls (5 repos) | 3 | yes, one per repo |
| 5 | Predicate evaluator skeleton + ConditionSet (AND/OR/N_OF_M) | 3 | yes |
| 6 | First 7 predicates: SubmittedAssignment, SubmittedQuiz, ViewedContent, OutcomeMastery, CurrencyThreshold, EarnedBadge, ReputationThreshold | 5 | yes, one per predicate |
| 7 | Mastery calc_method impls: `khan_spaced_retrieval`, `decaying_average`, `most_recent`, `highest`, `n_times`, `weighted_average` | 5 | yes, one per method |
| 8 | Effect: AwardCurrency (resolves currency by code; ledgered to wallet) | 4 | yes |
| 9 | System-currency seeder + tenant-creation hook + backfill one-shot | 4 | yes |
| 10 | `gamification.Emitter` service + rule dispatch loop | 5, 6, 7, 8 | no — serial integration |
| 11 | FERPA guard: enforce field-tag classification on Emit | 4, 10 | yes |
| 12 | xAPI emission for 20 core triggers: hook into existing submission, quiz, lesson, course completion services | 10 | yes, one verb cluster per agent |
| 13 | API handlers: `POST /api/v1/gamification/events` (testing hook), `GET /api/v1/gamification/currencies`, `GET /api/v1/users/:id/wallet` | 8, 10 | yes |
| 14 | Test fixtures + integration tests against `pgvector/pgvector:pg16` | 4–13 | yes |
| 15 | `TestSchemaParity_Wave1` updates + `make schema-diff` clean | 2, 3 | no — gates merge |

---

## Risks & mitigations

1. **JSONB-heavy schema** — predicate trees in JSONB are flexible but un-typesafe at the DB layer. *Mitigation*: schema-validate every `condition_set` insert via a Go-side validator before write; consider a `pg_jsonschema` extension check later.
2. **Event-bus backpressure** — emit-time evaluation could become slow under load. *Mitigation*: Wave 1 ships synchronous evaluation (good for correctness + debug). Wave 2 introduces an outbox queue for async fan-out once the load pattern is known.
3. **Schema-parity test drift** — adding 5 tables + 1 ALTER will trigger `TestSchemaParity_Wave1`. *Mitigation*: land models and migrations in the same PR; run `make schema-diff` and `make stale-cols` before commit (project guardrail).
4. **iCloud-sync dupes** — parallel-agent worktrees writing to overlapping paths will leave `* 2.go` artifacts. *Mitigation*: pre-commit sweep `find . -name "* 2.go" -o -name "* 2.sql"` per CLAUDE.md.
5. **Currency-code collisions across scope** — a course-scoped `coins` collides with a site-scoped `coins` from the learner's view. *Mitigation*: resolution order at lookup is `section > course > school > district > site`; first match wins. Document and surface in the (Wave 2) currency-create UI as a warning.
6. **Mastery calc_method explosion** — six methods × many outcomes × event history could get expensive. *Mitigation*: each `mastery/*.go` impl is pure and memoizable; cache `(outcome_id, user_id) -> MasteryState` keyed by `last_relevant_event_at`.

---

## What's explicitly out of scope for Wave 1

- Teacher-facing recipe builder UI → Wave 2
- Currency-create / currency-edit UI for teachers → Wave 2 (system seed only in Wave 1)
- Top-bar currency widget (Duolingo-style icon pills) → Wave 2 (front-end work)
- Badge issuance + Open Badges export → Wave 2
- Notification dispatch (only stubbed) → Wave 2
- Leaderboards → Wave 3
- Streaks → Wave 3
- Skill-tree visualization → Wave 3/4
- Adaptive item engine (Rasch/IRT) → Wave 4
- LTI 1.3 outbound, signed webhooks, conformant `/xapi/` endpoint → Wave 5
- Canvas integration of any kind — Paper LMS *is* the LMS. Migration-import tooling from existing Canvas tenants is a future Phase X (post-Phase 6).

---

## Definition of done

- All four migrations apply cleanly forward and backward against `pgvector/pgvector:pg16`.
- System currency seeder runs on tenant creation; backfill one-shot populates existing tenants. The four system currencies are present and `system_owned = TRUE` for every tenant.
- `TestSchemaParity_Wave1` passes.
- `go vet ./...` and `go test ./...` clean.
- The 20 core triggers emit `gamification_events` rows when their underlying services fire.
- A test rule (XP-award on `quiz.passed`) writes to `gamification_wallet_transactions` and updates `gamification_wallet_balances` when its trigger fires.
- A second test rule using a course-scoped custom `coins` currency proves the user-defined currency path end-to-end.
- A test of all six mastery `calc_method` impls — given the same event history, they produce the documented expected states (Khan: level transitions on spaced re-test; decaying-average: smooth numeric; most_recent / highest / n_times / weighted_average: their canonical behaviors).
- FERPA guard rejects any `Emit` that carries an `education_record`-classified field in a `non_PII` field-tag context (test fixture proves it).
- All public functions documented; one short `docs/research/gamification-2026-05/WAVE1-NOTES.md` captures any decisions taken inline during build.

---

## Recommended next action

Decisions are resolved; Boot.dev research is in flight. Once the Boot.dev report lands we'll know if any of its features need a new Wave 1 architectural primitive (e.g., **pets/companions** may need a new `gamification_companion_*` table, or it may just be a special-case effect type — depends on what the research finds). Until then, tasks 2–5 (migrations + models + repos + predicate scaffold) are safe to land as the first PR foundation.
