# Paper LMS Gamification — Master Synthesis

*2026-05-12. Merging three Claude research agents (01–03) with a parallel AI PRD-style brief (05). This is the canonical doc — start here.*

---

## TL;DR

Two independent research streams converged on the same architecture: **one unified rules engine over an xAPI-shaped event bus**, with a Brightspace-style shared predicate language used by everything (content gating, badges, XP, branching paths, agents, leaderboards). Paper LMS exceeds every system surveyed by adding three predicates Brightspace doesn't have (N-of-M, mastery-percentage-as-trigger, branching-as-explicit-effect), a **four-currency economy** (XP / Mastery / Gems / Reputation) that separates effort from FERPA-flagged mastery, and **reputation-gated capabilities** (Stack Overflow pattern) that turn engagement into platform power rather than just decoration. The compliance posture is non-negotiable: tenant mode flag (`K12 | HigherEd | Corporate | Professional`) and per-field FERPA classification are architectural, not configuration. Lead with white-hat Octalysis drives (CD1/CD3/CD5); gate every black-hat mechanic (CD6/CD7/CD8) behind explicit school-admin opt-in for under-13.

---

## 1. The architectural spine: xAPI event bus + unified rules engine

Every gamification-relevant action emits an **xAPI statement** (`actor + verb + object + result + context`). The internal event store is LRS-shaped. The rule engine subscribes to that stream; every effect (currency award, badge issuance, content release, agent fire, leaderboard update) is a consumer. External systems plug in via the same stream — LTI tools and webhooks in, signed webhooks out, conformant LRS at `/xapi/` for SCORM/cmi5 legacy. (Paper LMS *is* the LMS; there's no inbound Canvas bridge — events come from internal services calling `gamification.Emit()` directly.)

This collapses what Canvas/D2L/Moodle implement as three separate subsystems (overrides, release conditions, XP plugins) into one engine.

### The schema (PostgreSQL + JSONB)

```sql
-- The universal event store; mirrors xAPI shape
events (
  id uuid primary key,
  occurred_at timestamptz not null,
  actor_id uuid not null,           -- opaque internal ID, never email
  verb text not null,                -- 'completed' | 'mastered' | 'answered' | ...
  object_type text not null,
  object_id uuid not null,
  result jsonb,                      -- {score, success, completion, response, ...}
  context jsonb,                     -- {course_id, section_id, registration, ...}
  tenant_id uuid not null,
  policy_flags text[],               -- ['ferpa_protected', 'education_record', ...]
  source text not null               -- 'internal' | 'lti' | 'webhook' | 'migration_import'
);

-- Rules: the unified gating / awarding / branching language
rules (
  id uuid primary key,
  scope_type text not null,          -- 'site'|'district'|'school'|'course'|'section'
  scope_id uuid not null,
  audience_level text not null,      -- 'k5'|'68'|'912'|'higher_ed'|'corp'
  name text not null,
  enabled boolean default true,
  trigger_event jsonb not null,      -- OnEvent | OnSchedule | OnManualTrigger
  condition_set jsonb not null,      -- recursive AND/OR/N_OF_M predicate tree
  effects jsonb[] not null,          -- ordered list of effects
  cooldown_seconds int,
  max_per_window jsonb,              -- {window: 'day'|'week'|'lifetime', count: N}
  created_by uuid,
  created_at timestamptz default now()
);

rule_evaluations (
  rule_id uuid, user_id uuid, evaluated_at timestamptz,
  predicate_state jsonb, result boolean, effects_fired jsonb,
  primary key (rule_id, user_id, evaluated_at)
);
```

### Rule evaluation

- **Event-driven** for `OnEvent` triggers — subscriber on the `events` table fans out matching rules.
- **Scheduled cron sweep** (nightly + hourly) for date-based and inactivity predicates (D2L Intelligent Agents pattern).
- **Idempotency** on `event_id` — our own rules can re-fire on resubmission (e.g., regrade moving a learner to a different score band). Path reassignment is explicit, not an error.
- **Anti-farm** at evaluation time: `cooldown_seconds` and `max_per_window` are evaluated against the actor's prior `rule_evaluations` before any effect fires.

### Predicate vocabulary (24 atomic types)

Copy D2L Release Conditions verbatim; add the four extensions that beat Canvas and Brightspace. Predicates compose via `ConditionSet`.

| Group | Predicates |
|---|---|
| **Submission/Score** | `SubmittedAssignment(id, score_range)`, `NotSubmittedAssignment(id, by_date)`, `SubmittedQuiz(id, score_range)`, `ReceivedRubricRating(rubric, criterion, ≥level)` |
| **Content/Activity** | `ViewedContent(id)`, `CompletedContent(id)`, `ViewedAllInModule(module_id)`, `PostedInDiscussion(topic_id, min_n)` |
| **Mastery** | `OutcomeMastery(outcome_id, level, calc_method)` ← *Canvas-killer*, `KhanMasteryLevel(skill_id, ≥Familiar|Proficient|Mastered)` ← spaced-retrieval decay |
| **Economy** | `XPThreshold(amount, scope)`, `LevelThreshold(n)`, `EarnedBadge(id)`, `ReputationThreshold(amount)`, `CompletedChecklist(id)` |
| **Enrollment/Time** | `EnrolledIn(group\|section\|role)`, `DaysSinceEnrollment(n)`, `DaysSinceLastLogin(n)`, `DateWindow(start, end)`, `RelativeDateFromEnrollment(offset)` |
| **Composition** | `ConditionSet(op: AND \| OR \| N_OF_M, children: [...])` — recursive boolean tree, **N-of-M is first-class** |

### Effect types (8)

| Effect | Notes |
|---|---|
| `ReleaseContent(item_id \| module_id)` | Strict superset of Canvas Mastery Paths' assignment-override mechanism |
| `AwardCurrency(type: XP\|Mastery\|Gems\|Reputation, amount, multiplier?)` | With cheat-guard parameters; emits its own event |
| `AwardBadge(badge_id, evidence?)` | Internal default; external OB 3.0 export is a separate effect |
| `AdvanceRankOrLevel` | Derived from XP threshold; emit when crossing |
| `UnlockCapability(capability_id)` | NEW — Stack Overflow pattern (peer review, study room host, content propose, mentor) |
| `BranchPath(next_item \| path_label)` | Explicit DAG branching; Canvas can only do implicit 3-band fan-out |
| `EnrollInGroup(group_id)` | Auto-cohort by performance (remediation, enrichment) |
| `Notify(recipients, template, channel)` | In-app / email / push, frequency-capped per tenant mode |

---

## 2. The economy: user-defined currencies, four system-seeded

Khan Academy separates Energy Points (effort) from Mastery; the parallel research generalizes this to multiple parallel currencies. **Paper LMS goes further: currencies are user-defined (MyCred pattern), not a fixed enum.** A `gamification_currency_types` table lets each tenant, course, or section define unlimited currencies with custom names, icons, colors, decay rules, spendability, FERPA classification, and topbar visibility.

**Four currencies are system-seeded on every tenant.** These can be renamed (`display_label`, `icon`, `color`) but not deleted, because rules and capability unlocks reference them by `code`:

| System code | Default label | Earns from | Spendable | Monotonic | FERPA flag | Public surfaces |
|---|---|---|---|---|---|---|
| `xp` | "XP" | Effort: completions, attempts, time-on-task (capped) | No | Yes | `non_PII` | OK on leaderboards, level meters, rank progression |
| `mastery_points` | "Mastery Points" | Demonstrated skill: outcome mastery, rubric ratings, graded assessments | No | Yes | `education_record` | Private by default; **blocked from public leaderboards** by the data-access guard |
| `gems` | "Gems" | Gameplay: quests, surprise bonuses, achievement unlocks, perfect scores | Yes | No | `non_PII` | OK |
| `reputation` | "Rep" | Social: helpful answers, peer reviews, kudos received | No | Yes | `non_PII` | OK — gates capability unlocks |

**Teachers add their own currencies on top.** Example from the user's pre-AI LearnDash setup:

| Code | Label | Spendable | Monotonic | Description |
|---|---|---|---|---|
| `coins` | "Coins" | Yes | No | Earned for homework completion. Spend in the class shop. |
| `class_bucks` | "Class Bucks" | Yes | No | Teacher-granted behavior currency (Class Dojo replacement). |

Custom currencies are scoped (`site` / `district` / `school` / `course` / `section`). A learner enrolled in multiple courses each with their own `coins` currency carries multiple balance rows — currencies are identified by UUID internally, but rules reference them by `code` for portability.

**Hard rules:**
- **Never** sell `gems` or any spendable currency for real money to under-18. Earned-only. Loot boxes for purchase are banned for minors in BE/NL and increasingly restricted EU-wide.
- **`mastery_points` is FERPA-blocked from public leaderboards** at the data-access layer, regardless of any teacher misconfiguration. The `ferpa_classification = 'education_record'` flag is sticky.
- `reputation` is special — its semantics are locked to the capability ladder. Teachers can rename the label but not the code; otherwise rules referencing `ReputationThreshold(N)` break.
- Anti-abuse: every transaction immutable-logged (MyCred pattern). Rule-level caps, daily caps per source, instructor approval for transactions above threshold.

**Top-bar UI (Wave 2):** Duolingo-style icon pills, ordered by `display_order`, filtered by `visible_in_topbar`. Click a pill → drawer with that currency's transaction history. Icon + color + label all come from `gamification_currency_types`, no hardcoding.

---

## 3. Reputation-gated capabilities (Stack Overflow pattern)

Reputation thresholds unlock **platform powers**, not decorations. This is the engagement loop that converts a 1-year course into a 10-year community.

| Threshold | Capability |
|---|---|
| Rep 3 | Submit peer reviews on graded artifacts |
| Rep 5 | Host a study room (real-time session for ≤8 peers) |
| Rep 10 | Propose course content edits (teacher-moderated queue) |
| Rep 15 | Author practice problems (teacher-vetted; goes into the adaptive bank) |
| Rep 20 | Mentor designation — paired with new learners, visible on profile |
| Rep 50 | Alumni community access, prestige reset option |

Tenant admins configure thresholds. Defaults above are for HigherEd; K-12 defaults are higher and most capabilities are teacher-approved.

---

## 4. The trigger inventory

~100 canonical event types across nine categories. The full list lives in **[01-claude-wp-stack.md §7](./01-claude-wp-stack.md#7-unified-trigger-taxonomy--paper-lms)** (95 triggers) and **[05-parallel-ai-prd.md PART 2](./05-parallel-ai-prd.md#part-2--master-trigger-taxonomy)** (overlapping but adds Negative/Recovery + External/API categories).

Merged categories:

| Category | Approx count | Examples |
|---|---|---|
| A. Learning Progress | 14 | course.enrolled, lesson.completed, module.completed, learning_objective.progressed, path.branched |
| B. Assessment Mastery | 15 | quiz.passed_with_threshold, question.answered_correctly (the adaptive math hook), assignment.submitted_on_time, quiz.improved_on_retake |
| C. Mastery & Skill | 7 | outcome.mastered, skill.demonstrated, competency.achieved, mastery.recovered, skill_tree.node_unlocked |
| D. Time / Streak | 10 | session.daily_login, streak.extended, daily_goal.met, streak.frozen, study_session.completed |
| E. Social | 15 | discussion.reply_posted, peer_review.submitted, kudos.received, friend.streak_started, mentor.session_completed |
| F. Content Creation | 12 | content.discussion_post, content.wiki_edit, content.notes_published, content.flashcard_set_created |
| G. Engagement Depth | 10 | video.watched (≥80%), office_hours.attended, poll.voted, survey.completed |
| H. Instructor / Admin | 10 | award.manually_granted, recognition.given, exception.granted, points.bulk_assigned |
| I. Negative / Recovery | 8 | streak.broken, inactive.7_days, at_risk.detected, quiz.failed_twice (→ path.remediation_assigned) |
| J. External / API | 6 | webhook.received, lti.tool_event, xapi.statement.received, canvas.live_event, scheduled.cron |

Each trigger carries the same xAPI envelope: `event_id`, `event_time`, `actor`, `object`, `result`, `context`, `policy_flags`, `signature`.

---

## 5. Compliance layer (architectural, not configuration)

### Tenant mode flag — first thing the system reads

`K12 | HigherEd | Corporate | Professional` drives every default toggle. K-12 mode is the safest superset:
- COPPA mode auto-applied for any user under 13.
- Friend streaks, public leaderboards, Open Badges export, behavioral profiling — all OFF by default.
- Verifiable parental consent required before any PII collection.

### FERPA field-classification taxonomy

Every data field tagged: `directory_information | education_record | non_PII | instructor_metadata`. Disclosure pathways enforced at the **data-access layer**, not the UI layer.

### Per-learner leaderboard opt-out

Confirmed whitespace (parallel research Appendix A.1): Brightspace's June 2025 `MaskUsernames` config is admin-only, off by default, no per-learner toggle. Paper LMS ships per-learner opt-out as a first-class control — opting out does **not** reduce XP or remove the learner from awards. This survives FERPA audits where Brightspace's widget may not.

### Open Badges posture

- **Internal badges by default for all tenants.**
- **OB 3.0 export, opt-in, eligible students only** (18+ or parental consent on file).
- **DID-based earner identifier**, never email.
- **No under-13 wallet accounts.** School is the COPPA-consenting party under the FTC School Authorization model. The badge artifact stays server-side under the school's account; the family receives the badge on request through the LMS, not through a third-party wallet.
- Consent checkpoint is a **hard gate** in the export flow, logged as an audit event.

### COPPA 2.0 watch

Passed US Senate unanimously 2026-03-05, pending House. If enacted, extends protections to under-17. Would materially change every default in the tenant matrix. Revisit the entire toggle matrix on enactment.

### Audit log

Every manual award, revocation, override, consent grant, and PII disclosure is logged with actor, reason, and timestamp. Immutable (append-only).

---

## 6. Adaptive engine (the math word-problem redux)

Your old pre-AI randomized math word-problem engine + MyCred per-question coins hit four motivational drives simultaneously: competence (SDT), flow (Csikszentmihalyi), CD3 immediate feedback (Octalysis), CD7 unpredictability (Skinner). The Paper LMS rebuild:

### Algorithm — patent-free

- **Rasch / 1PL IRT** with maximum-Fisher-information item selection. Pure 1950s academic math, unencumbered. Use `catR` / `mirt` (R) or `py-irt` (Python) as the calibration library.
- **Field-test item embedding** (NWEA's own technique): each session includes a small number of unscored field-test slots; calibration data accumulates without burdening the learner. Once an item has ~100 responses, promote from field-test to operational.
- **AI-generated items** vetted by teacher review queue. AI does what randomization did, with infinitely more variety and pedagogical control. Items earn rep when teacher-approved (powers the Rep 15 capability above).
- **Target ~80% expected success per learner per item** — the flow channel. Difficulty bands per outcome: easy / on-grade / stretch. Auto-bump after 3 correct in a row.

### Anti-IP-encumbrance checklist

- **Avoid** "MAP", "RIT", "Rasch Unit" as product or score-scale names.
- **Commission USPTO FTO search** on NWEA and HMH Education Company assignees before launch. Budget **$5K–$15K** for the attorney opinion. Specific terms to search: "item selection", "content proximity", "blueprint constraints in CAT", "off-grade item selection". NWEA's Enhanced Item Selection Algorithm and Content Proximity methodology are NWEA-proprietary innovations — implement your own; don't clone theirs.
- **Move to 2PL** once response volume per item exceeds ~300.

### Frustration detection

Time-on-task variance + abandonment + consecutive-wrong + hint-pull frequency. On detection: auto-offer scaffold, easier alternate item, suggest a break, ping teacher dashboard. On boredom detection (very fast, near-zero errors): bump difficulty, offer stretch challenge.

### Per-question coin award

The `question.answered_correctly` event carries `result.score` and the item's calibrated difficulty. A rule awards variable XP (5–25, hidden distribution; CD7) scaled by difficulty. Mastery Points accrue separately on the same event, gated by the outcome alignment. Gems drop occasionally on mastery completion (earned, not purchased).

---

## 7. Design principles — the ship/no-ship checklist

Six principles that survived independent cross-checking by both research streams:

1. **Tie XP to demonstrated mastery, never time-on-page.** Overjustification effect (Deci 1971; Deci-Koestner-Ryan 1999) crowds out intrinsic motivation in school-aged children. XP per minute is a dark pattern.
2. **Surprise > promise.** Unexpected rewards don't crowd out intrinsic motivation; if-then rewards do. Variable bonus XP, mystery boxes on mastery (never purchase), surprise badges.
3. **Relative leaderboards, ≤30 cohorts, weekly reset.** Duolingo settled on 30 per league for a reason. Global all-time leaderboards demotivate the bottom 80% permanently (Li 2024, JCAL).
4. **Streaks auto-freeze on weekends, holidays, offline status, and IEP/illness flags for K-12.** Frame as "learning days this month," never "don't break the streak."
5. **Black-hat mechanics (CD6/7/8) require explicit school-admin opt-in for under-13.** Default OFF for K-8. Scarcity, loss-aversion, FOMO are dark patterns when targeted at developing executive function.
6. **Three-level toggle hierarchy: site → course → section.** A 5th-grade teacher must be able to disable mechanics their middle-school colleague leaves on.

Full K-5 → Corporate toggle matrix lives in **[03-claude-behavioral.md § K-12 vs Higher Ed Toggle Matrix](./03-claude-behavioral.md#k-12-vs-higher-ed-toggle-matrix)**.

---

## 8. Endgame design (the multi-year retention loop)

After a learner has earned every badge and reached every rank, what's left? This tier is missing from every LMS surveyed and is a strategic moat:

- **Prestige reset** — voluntary level reset with a permanent badge marker. Each prestige unlocks cosmetic distinctions and additional capability slots.
- **Mentor designation** at Rep 20 — paired with new learners, visible on profile, earns reputation through mentee outcomes.
- **Cohort leadership** — host a study group, run an event series, propose a curriculum thread.
- **Content authorship** — practice items, lesson revisions, peer-reviewed glossary entries. Items earn micro-royalties of Gems when used.
- **Alumni network** — graduated learners retain platform access at reduced privileges; can mentor and contribute content.

---

## 9. Ship roadmap (5 waves, indicative)

### Wave 1 — Foundations (Phase 6 start, ~4–6 weeks)

- `events` table with xAPI-shaped schema; event-bus subscriber infrastructure.
- `rules` + `rule_evaluations` tables; predicate evaluator for the 24 atomic types; recursive `ConditionSet` parser.
- `currency_types` table (MyCred-pattern) + wallet (`balances`, `transactions`) referencing currency by UUID, rules referencing by code. Four system-seeded: `xp`, `gems`, `mastery_points`, `reputation`. Teachers add their own.
- Six mastery `calc_method` impls (`khan_spaced_retrieval`, `decaying_average`, `most_recent`, `highest`, `n_times`, `weighted_average`) — selectable per outcome and overridable per rule.
- Tenant mode flag + FERPA field-classification taxonomy enforced at the data-access layer.
- xAPI statement emission for the first 20 core triggers (A + B + small slice of D). All events fire from internal Paper LMS services — no Canvas inbound bridge.

### Wave 2 — Teacher tools + core rewards (~4 weeks)

- Recipe builder UI (Uncanny Automator pattern) — visual `WHEN trigger AND condition_set THEN effects` composer. Templates library with 50 pre-built recipes.
- Internal-only badges (default). OB 3.0 export gated behind eligible-student/parent consent; never automatic.
- Core 37 triggers wired: Learning Progress (14) + Assessment Mastery (15) + Streak (8 minus 2 social).
- Manual award console with audit log.
- Per-learner leaderboard opt-out + read-only learner dashboard.

### Wave 3 — Social + leaderboards (~4 weeks)

- Relative leaderboard widget (top 5 within ±N of viewer); cohort-scoped, weekly reset, ≤30 per cohort.
- Duolingo-style weekly leagues (HigherEd+ default; opt-in K-12).
- Reputation system + capability ladder (peer review, study room host, content propose, mentor).
- Streak engine with auto-freeze (weekends/holidays/IEP), Friend Streaks (HigherEd+ default).
- Frustration & boredom detection in quiz engine; intervention hooks.

### Wave 4 — Adaptive engine (~6 weeks)

- Rasch / 1PL IRT item-selection service. `mirt` or equivalent under the hood.
- Item bank with field-test embedding; auto-promotion on ~100 responses.
- AI-assisted item generation + teacher review queue (powers Rep 15 capability).
- Per-question economy hook: variable XP (5–25, hidden distribution), Mastery Points by outcome alignment, occasional Gems on mastery.
- USPTO FTO search complete before launch.

### Wave 5 — Endgame + interop (~4 weeks)

- OB 3.0 export for 18+ (DID-based earner identifier; consent hard-gate; audit-logged).
- Prestige resets, mentor designation, alumni community shell.
- Skill-tree visualization (Duolingo path + Khan mastery decay semantics).
- ML-light at-risk detection; instructor early-warning dashboard.
- Outbound webhook framework (JWT signing, HMAC verify, retry policy, DLQ).
- Conformant LRS at `/xapi/`; LTI 1.3 / Advantage compliance.
- Procedural endgame challenges.

**Deferred to a future Phase X (post-Phase 6):** Pet / Companion system. Confirmed 2026-05-12 — the user wants pets eventually but acknowledges it deserves a dedicated design pass because pets carry their own economy: per-species boosts (a cat provides focus music; a dragon provides XP multipliers; etc.), multipliers, perks, evolution/leveling, cosmetic equipment, mood/hunger state machines. Not Wave 5; its own future wave.

---

## 9b. Boot.dev integration

The user named Boot.dev as the engagement-quality target and asked for every feature replicated. Full research at [06-claude-bootdev.md](./06-claude-bootdev.md). Distilled deltas to the architecture above:

### Items are currencies — no new schema

Boot.dev's items (Baked Salmon, Seer Stone, Frozen Flame, Ember, Potion) all model cleanly as `gamification_currency_types` rows with `spendable=true`, `monotonic=false`, custom icons. The wallet handles consumption naturally — no `items` table needed. **One architectural fit verification passed.**

What *does* need a new primitive: **active modifiers** (Potion's 1h XP buff). That's a tiny `active_effects(user_id, kind, multiplier, expires_at)` table. Wave 3 add.

### New primitives Boot.dev forces into the roadmap

1. **Loot tables** (Wave 2.5/3) — `loot_table`, `loot_table_entry`, server-side weighted RNG with seeded auditability. Chests are the structural backbone of Boot.dev's reward variance (CD7). xAPI event `chest_opened` with provenance.
2. **Streak protection state machine** (Wave 3) — Ember + Frozen Flame two-tier consumption is the user-safe streak design taken to its conclusion. `streak_protection(user_id, kind, charges, fifo_order)` consumed before declaring a streak broken. Plus the "above-and-beyond" daily-XP threshold that auto-mints an Ember.
3. **Leagues** (Wave 3) — `league`, `league_membership`, `league_season` + matchmaker job. 25-person pods, 4-week seasons, promotion/demotion. Different from rank.
4. **Community boss-fight events** (Wave 3, **confirmed 2026-05-12, +2 weeks budgeted**) — `community_events`, `community_event_contributions`, `narrative_chapter` tables, live-feed websocket, aura-XP multiplier engine. **The headline CD1 (Epic Meaning) differentiator** — the user described this as the mechanic "users will love and remember 30 years after." A web novel whose chapters advance only when the community wins is the single most distinctive mechanic in the entire research corpus and the strongest moat against Canvas / Brightspace / Moodle.
5. **Karma** (Wave 3) — community-engagement currency aggregated from discussion-reply / kudos / helpful-answer events. Source-specific currency_type with FERPA-safe display.
6. **Public profile renderer** (Wave 3) — read-side projection at `/u/<handle>` with rank frame, completed tracks, capstone link.
7. **Procedural challenges** (Wave 5) — endgame practice generator. Hooks into the Wave 4 adaptive engine.

### Boot.dev signatures Paper LMS adopts

- **Sharpshooter Spree** — 15-in-a-row correct triggers a chest. State column on enrollment + simple predicate. Reward quality, not quantity.
- **"5 days a week" framing** — `streak_protection` makes the daily streak forgiving by design. Default config: 2 auto-Frozen-Flames per month + Ember from above-and-beyond days.
- **Activity-source flexibility for streaks** — Boot.dev counts GitHub commits toward the streak. Paper LMS analog: tenant-configurable list of streak-eligible events (`lesson.completed`, `assignment.submitted`, `course.contributed_artifact`, etc.). Real work counts.
- **Economic friction as anti-cheat** — XP penalties for viewing solution (75%) or AI tutor pre-completion (50%), with item bypasses purchasable from gems. The XP penalty IS the speed-bump; no server-side timer needed. Maps to standard predicate-engine rules.
- **Mobile is focus-first, not castrated** — confirmed 2026-05-12. We don't disable features on mobile for anti-cheat reasons (the soft anti-cheat is purely economic friction + cadence gates, which work fine on mobile). But the mobile *UX* defaults to a learning-first layout that hides game-economy chrome (gem store, quest browser, boss-fight live feed) behind a single drawer, so the small screen stays focused on the lesson. Users can opt into full chrome via a settings toggle. Wave 2 UX design call, not a schema concern.
- **Rank-name capability roles** — Apprentice → Archmage style ladder. Tenants pick their own theme; the capability ladder underneath is the same as Reputation's.

### Pets — deferred to a future Phase X (post-Phase 6)

**Confirmed deferred 2026-05-12.** The user wants a pet/companion system eventually but acknowledges it deserves its own design pass because pets aren't just cosmetic — they carry their own economy of per-species boosts, multipliers, perks, evolution/leveling, equipment, and mood/hunger state machines. Examples the user named: a cat that provides focus music; (implied) per-species buffs that interact with the XP and item systems.

Architectural notes for the future wave (not Wave 5):
- Pet state lives in its own tables (`pet`, `pet_action_log`, `pet_inventory`, `pet_species`, `pet_evolution`).
- Pet effects compose through the existing `active_effects` table — a pet's "focus music aura" is the same primitive as a Potion's 1h XP buff.
- Pet cosmetics ride on the same icon/color machinery as currencies.
- Pet feeding/care actions emit xAPI events and are themselves rule triggers (e.g., "fed pet" → small XP, "neglected pet" → mood drops).
- Pairs naturally with the in-platform AI tutor module: the pet can *be* the tutor's persistent embodiment, or be a separate companion.

Out of scope until then. Don't pre-build hooks for it.

---

## 10. Open questions to resolve before Wave 1

These are the items neither research stream fully closed; they need decisions or external work before the schema is locked.

1. **USPTO FTO search on NWEA / HMH** — commission attorney opinion. Confirms the Rasch + Fisher-information approach is clean and identifies any encumbered claim language to avoid. Budget $5K–$15K. **Blocks Wave 4 launch, not Wave 1 schema.**
2. **Legal review of effort-derived metrics on public leaderboards** — XP from logins/attempts is less FERPA-settled than grade-derived metrics. Review with institutional counsel for an example K-12 deployment to set defaults conservatively. **Blocks Wave 3.**
3. **COPPA 2.0 enactment status at build time** — if it passes the House before Wave 3, every default in the tenant matrix shifts (under-13 → under-17). Track at every wave boundary.
4. ~~Canvas Live Events transport~~ — **resolved 2026-05-12: removed entirely.** Paper LMS replaces Canvas; events come from internal services. Canvas import tooling is a future Phase X concern, not Wave 1.
5. **Mastery decay semantics: Khan-style spaced-retrieval vs Canvas decaying-average** — both are supported in the `OutcomeMastery` predicate's `calc_method` parameter, but the default per audience needs a pedagogical call. **Recommendation:** Khan-style (`KhanMasteryLevel`) for K-12 formative; Canvas decaying-average for HigherEd graded.
6. **AI-generated item review queue throughput** — how many items per week can a teacher realistically vet? Affects whether AI item gen is a Wave 4 ship or a Wave 5 ship.

---

## 11. Where the two research streams diverge or correct each other

Mostly the parallel AI stream **extends** the Claude agent findings — there are no hard contradictions. The deltas worth flagging:

| Area | Claude agents | Parallel AI | Resolution |
|---|---|---|---|
| Event bus | "Subscribe to your existing submission/grade/view events" | xAPI statements as the **internal substrate**, not export | **Adopt parallel's framing.** xAPI is the spine. |
| Currency count | One (XP), with mastery as a separate trigger source | Four (XP / Mastery / Gems / Reputation) | **Adopt parallel's four-currency model.** FERPA + pedagogical win. |
| Teacher UX | "Visual predicate composer mirroring D2L's release-conditions modal" | Recipe builder (Uncanny Automator pattern) — `WHEN X THEN Y` recipes with templates library | **Both are right.** Recipe = trigger + condition_set + effects. Same data model; recipe library is the UX. |
| Capability unlocks | Not addressed | Reputation gates platform powers (Stack Overflow) | **Adopt as a new effect type** (`UnlockCapability`). |
| Endgame | Not addressed | Prestige, alumni, mentor — multi-year retention | **Adopt as Wave 5.** |
| Brightspace leaderboard privacy | "Does the widget support student opt-out?" — couldn't confirm | Confirmed: admin-only `MaskUsernames` config (June 2025), no per-learner opt-out | **Per-learner opt-out is genuine whitespace.** Ship Wave 2. |
| Canvas Mastery Paths timing | "Fires on grade-posted" | Confirmed; further gated by Grade Posting Policy (Auto vs Manual) | Resolved 2026-05-12 — N/A. Paper LMS replaces Canvas; the analog (Paper's own grading service emits `graded` / `mastered`) is in-process and exactly-once. Insight preserved for any future Canvas-migration-import tool. |
| Duolingo Energy | "Still in A/B test as of mid-2025" | Fully rolled out on mobile by May 2026; no rollback even for Super subscribers | **Energy is a cautionary tale, not inspiration.** Any failure-friction mechanic depletes on disengagement, never on correct performance. |
| OB 3.0 under-13 | "Opt-in only" | No production wallet handles under-13 at all; school is the COPPA-consenting party | **Internal-only badges for K-12 is the only legally defensible posture.** Build the OB 3.0 export only for 18+. |
| Adaptive engine | "Verify with parallel AI" — flagged as a research gap | Rasch 1PL, patent-free, $5K–$15K FTO budget recommended | **Adopt Rasch 1PL as the Wave 4 algorithm.** Commission the FTO search before launch. |

---

## 12. References

- **[README.md](./README.md)** — reading order and high-level conclusions.
- **[01-claude-wp-stack.md](./01-claude-wp-stack.md)** — WordPress MyCred/GamiPress/BadgeOS/LearnDash/BuddyBoss with 95-trigger taxonomy.
- **[02-claude-lms-native.md](./02-claude-lms-native.md)** — Canvas / Brightspace / Moodle / Khan / Schoology with 24 atomic primitives.
- **[03-claude-behavioral.md](./03-claude-behavioral.md)** — Octalysis 8 drives, Duolingo old/new, SDT, Flow, streak design, dark patterns, K-5→Corp toggle matrix.
- **[05-parallel-ai-prd.md](./05-parallel-ai-prd.md)** — Parallel AI's PRD-style brief with 200+ triggers, 12 feature specs, and Appendix A verifying Brightspace leaderboard masking, Canvas Mastery Paths timing, OB 3.0 under-13 reality, Duolingo Energy status, NWEA/Rasch patent status.

All citations in the source documents are URL-linked inline.

---

*End of synthesis. Last updated: 2026-05-12.*
