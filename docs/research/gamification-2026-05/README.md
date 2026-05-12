# Paper LMS Gamification Research — May 2026

Two-stream research into building the most engaging LMS gamification system on the planet. Goal: a Canvas-parity LMS (Paper LMS) that exceeds Brightspace's adaptive engine, Duolingo's engagement loops, and the WordPress LearnDash+MyCred+BuddyBoss stack the founder previously ran — while staying FERPA-native and ethically defensible for K-12.

## Read order

1. **[SYNTHESIS.md](./SYNTHESIS.md)** — the canonical document. Merged architectural decisions, four-currency economy, predicate vocabulary, ship roadmap, and reconciliation between the two research streams. **Start here.**
2. **[PHASE6-WAVE1-PLAN.md](./PHASE6-WAVE1-PLAN.md)** — concrete implementation plan for Wave 1: migrations 000032–000035, Go module layout, predicate evaluator design, Canvas Live Events bridge, 15-task breakdown, definition of done.
3. **[05-parallel-ai-prd.md](./05-parallel-ai-prd.md)** — the parallel AI's PRD-style brief with 200+ triggers, 12 feature specs, and a verified Appendix A covering Brightspace leaderboard masking, Canvas Mastery Paths trigger timing, Open Badges 3.0 under-13 reality, Duolingo Energy status, and NWEA/Rasch patent status.
4. **[01-claude-wp-stack.md](./01-claude-wp-stack.md)** — Claude agent on MyCred, GamiPress, BadgeOS, LearnDash hooks, BuddyBoss social triggers, FERPA-safe badging. 95-trigger taxonomy.
5. **[02-claude-lms-native.md](./02-claude-lms-native.md)** — Claude agent on Canvas Mastery Paths + Outcomes, Brightspace Release Conditions + Intelligent Agents + Awards, Moodle Level Up XP, Khan Academy mastery, Schoology, Google Classroom. 24 atomic primitives.
6. **[03-claude-behavioral.md](./03-claude-behavioral.md)** — Claude agent on Octalysis 8 Core Drives, Duolingo old vs new, SDT, Flow, streak design, variable rewards, leaderboard research, dark-pattern avoidance, K-5 → Corporate toggle matrix.
7. **[06-claude-bootdev.md](./06-claude-bootdev.md)** — Claude agent on Boot.dev: every gamification mechanic (XP/curve, ranks, gems+items, chests/loot tables, streaks with Ember/Frozen Flame, daily quests, leagues, boss fights with narrative canon, Boots AI tutor, capstones, anti-cheat-via-economic-friction, Pro gating). User asked for every Boot.dev feature replicated.

## Convergent conclusions

Both research streams independently arrived at:

- **One unified rules engine** with a shared predicate language (Brightspace pattern) — the same conditions gate content, fire scheduled agents, award XP, issue badges, and branch paths.
- **N-of-M boolean trees** + **mastery-percentage as a trigger source** + **explicit branching as an effect** are the three things Paper LMS adds to beat Brightspace and Canvas.
- **FERPA-native architecture** — Open Badges 2.0 leaks PII via email; OB 3.0 with DIDs + opt-in export is the only legally defensible posture for K-12.
- **White-hat first** — lead with Octalysis CD1/CD3 (epic meaning, creativity & feedback); make CD6/CD7/CD8 (scarcity, unpredictability, loss) opt-in by school admin for under-13.
- **Relative leaderboards, ≤30 cohorts, weekly reset** — global all-time leaderboards permanently demotivate the bottom 80%.
- **Streaks auto-freeze on weekends + holidays for K-12**; never use loss-framed notifications for under-13.
- **Tie XP to demonstrated mastery, never time-on-page** — overjustification effect crowds out intrinsic motivation in school-aged children.

## What the parallel research added

Material extensions to the Claude-agent findings:

1. **xAPI/cmi5 as the internal event substrate**, not just an export format. Every gamification event is an xAPI statement; the rule engine is an LRS subscriber.
2. **Four currencies, not one**: XP (effort, non-PII, public-leaderboard-safe), Mastery Points (FERPA-protected), Gems (spendable, never purchased for under-18), Reputation (Stack Overflow pattern — gates platform capabilities).
3. **Tenant mode flag** (`K12 | HigherEd | Corporate | Professional`) as a first-class architectural concept that drives every default toggle.
4. **Recipe builder UI** (Uncanny Automator pattern) as the primary teacher-facing surface for composing rules.
5. **Reputation-gated capabilities** as a new effect type — earn the right to peer-review, host study rooms, propose course content, mentor.
6. **Endgame mechanics** as a separate design tier — prestige resets, alumni networks, mentor designations.
7. **NWEA MAP uses the Rasch model (1PL IRT)** — pure 1950s academic math, patent-free. The proprietary parts are the RIT scale, item bank, and item-selection optimizations. Start with `catR` / `mirt` / `py-irt`, budget $5K–$15K for an FTO search on NWEA/HMH assignees before launch, avoid "MAP" / "RIT" / "Rasch Unit" as product names.

## What the parallel research corrected

- **Brightspace `MaskUsernames` config was added June 2025** (admin-only, off by default, no per-learner opt-out). The earlier claim that Brightspace had no leaderboard privacy controls was outdated. Per-learner opt-out remains genuine whitespace.
- **Canvas Mastery Paths trigger timing is gated by the Grade Posting Policy** — Manual posting blocks branching until the instructor clicks "Post Grades." This is a non-obvious UX trap.
- **Duolingo Energy fully rolled out on mobile by May 2026, no rollback**. Even Super/Max subscribers can't opt back into Hearts. "No daily limit on free learning" is the strongest available competitive differentiator.
- **No production-grade under-13 wallet UX exists** anywhere in the OB 3.0 ecosystem. Credly and Canvas Badges both gate under-13 entirely; the school is the COPPA-consenting party under the FTC's School Authorization model. Internal-only badges remain the only defensible default for K-12.
- **COPPA 2.0 passed the US Senate unanimously on 2026-03-05**, pending House. If enacted, extends protections to under-17 — would materially change wallet, leaderboard, social-feature, and streak design for the entire teen population.

## Caveats

- The Claude agent reports were generated 2026-05-12; the parallel AI's brief is dated the same month. Web sources cited inline within each.
- Patent search for NWEA / HMH adaptive-engine claims was not exhaustive in either stream. Commission a formal USPTO FTO search before any commercial adaptive-engine deployment.
- The trigger taxonomy is a snapshot; MyCred and GamiPress ship monthly. Treat as directionally complete, not feature-frozen.
- All FERPA / COPPA interpretation in these documents is operational guidance, not legal advice. Review with institutional counsel before launch.
