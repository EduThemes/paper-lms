# LMS Gamification Research Brief & PRD-Style Feature Specification

*Parallel AI research stream, May 2026. For a Canvas-LMS-compatible custom platform serving K-12, higher education, corporate, and professional learners. Provided by the user to be merged with the Claude agent research streams (01–03).*

---

## TL;DR

- **The current LMS gamification market is fragmented and mediocre.** Canvas, Brightspace, and WordPress LMSes all expose primitive badge-and-points scaffolding (Canvas via Badgr/Mastery Paths, Brightspace via the Awards tool + Release Conditions, WordPress via MyCred/GamiPress/BadgeOS); none deliver an integrated, Duolingo-class engagement system, and most ignore the right-brain intrinsic drives that actually sustain learning. The white-space opportunity is a **trigger-first, Octalysis-balanced, FERPA-native gamification engine** that treats every learning event as a streamable signal and lets instructors compose mechanics like Lego.
- **Build the system around an event bus, not a feature list.** Canvas Live Events already emits 30+ webhook event types (submission_created, grade_change, quiz_submitted, etc.); xAPI/cmi5 provides a portable verb-noun-object vocabulary; MyCred and GamiPress prove that a hooks/triggers architecture scales to hundreds of native events. The right architecture is: **Source events → normalized xAPI statements → rule engine → reward + notification + analytics fan-out**. Everything else (points, badges, ranks, leaderboards, streaks, master paths) becomes a consumer of that bus.
- **FERPA, COPPA, and audience mixing are the hard constraints, not afterthoughts.** Public Open Badges 2.0 assertions, leaderboards displaying student names, and any gamification record tied to identifiable students are PII and subject to FERPA's consent and disclosure rules. The product must ship with a "Compliance Mode" that defaults K-12 deployments to anonymized/cohort-scoped leaderboards, internal-only badges (with optional opt-in Open Badges export at the eligible-student age), and per-tenant data-minimization controls.

---

## PART 1 — Research Brief

### 1.1 WordPress Gamification Plugins

**MyCred** is the most mature WordPress points-and-rewards engine. Its model is: *points types* (you can run multiple parallel economies — credits, gems, coins), *hooks* (preset events that award points), *badges* (achievement records keyed off the points log), and *ranks* (tiered statuses keyed off balances or sequential requirements). The Hooks system is the trigger backbone — login, content viewed, content published, comment, referral, video watched, link clicked, BuddyPress activity, WooCommerce purchase, and many more. Every point transaction is written to a centralized log, which is what badges, statistics, and anti-abuse limits read from. Notable add-ons: Notifications Plus, Email Notifications (with separate triggers for rank promotion/demotion, manual rank assignment, minimum-balance alerts), Zapier hooks (for trigger orchestration to external systems), Daily Login Rewards, Birthdays Plus, Time-Based Rewards, Coupons, buyCred (real-money point purchase), Sell Content, and an Open Badges add-on that supports baked Open Badge images, evidence URLs, single badge pages, and validation via third-party verifiers. The Open Badges output is the FERPA risk surface — public assertion URLs expose user identifiers. MyCred's LearnDash integration is community-driven; deeper triggers typically come via the LearnDash add-on or via Uncanny Automator.

**GamiPress** is the more flexible competitor. It separates *points types*, *achievement types* (you define them — badges, quests, missions, stamps), and *rank types* (level, grade, belt, etc.), each as a WordPress custom post type with full REST API endpoints at `/wp/v2/{type_slug}`. The Rest API Extended add-on adds dedicated POST endpoints for `award-achievement`, `revoke-achievement`, `award-rank`, `revoke-rank`, `upgrade-rank`, `downgrade-rank`, and per-requirement award/revoke — making GamiPress effectively a headless gamification microservice. GamiPress's LearnDash integration is granular: "complete any/specific quiz with minimum percent grade," "complete a quiz of a category/tag," "submit an essay for a quiz," "mark a topic incomplete," "review a course," and similar verb-style events. Integrations also exist for Tutor LMS, Sensei, BuddyBoss, BuddyPress, PeepSo, WooCommerce, Eventin, WP Ulike, and dozens more. GamiPress ships with WordPress personal-data export/deletion hooks (the foundation for GDPR-grade — though not FERPA-specific — compliance).

**BadgeOS** is the older sibling and was historically the bridge between WordPress and Credly/Mozilla Backpack Open Badges. Its trigger model is similar (achievement types, steps, points) but its ecosystem is thinner today; GamiPress was forked from the same lineage by some of the same contributors and has effectively surpassed it. BadgeOS still ships an LRS/xAPI bridge through its Community add-ons, but is no longer the default choice.

**Uncanny Automator** is the orchestration layer of the WordPress LMS gamification stack — a no-code trigger router that connects LearnDash, LearnDash Achievements, GamiPress, MyCred, BuddyPress, WooCommerce, Zoom, Slack, Twilio, Google Sheets, and 150+ other apps. Its recipes are explicit `When X happens → Do Y` pairs and are the de-facto pattern for "if a learner passes a quiz with >80%, award badge AND post in Slack AND add to a Google Sheet AND send a certificate." Any custom LMS should expose an Automator-equivalent recipe builder — that pattern is what instructors actually want.

**FERPA posture of WordPress plugins is weak.** None of these plugins were designed for FERPA; they are GDPR-aware (data export/deletion), but they freely write to public-facing leaderboards, allow Open Badge sharing by default, and surface user identifiers in shortcodes and blocks. A FERPA-compliant deployment requires explicit configuration: disable public leaderboards, restrict Open Badge export to opt-in/eligible students, and treat the points log as a controlled education record.

### 1.2 Canvas LMS Gamification Features

Canvas's native gamification is intentionally minimal. **Mastery Paths** is the closest thing to a built-in adaptive engine: any graded assignment, graded discussion, or graded quiz can be a "source item," and Canvas releases conditional content (assignments, quizzes, pages, discussions) based on scoring ranges. Three ranges per source item are supported, with optional `And`/`Or` branching to let students choose among parallel conditional items. Limitations are real: practice quizzes, ungraded surveys, and external tools cannot be source items; due dates for conditional items are not applied automatically; Mastery Paths is not integrated with the Outcomes/Mastery Gradebook framework; and changes to a path after grading do not retroactively re-assign. Many institutions use Mastery Paths as a primitive gamification engine ("score >90 → unlock bonus mission"), but it's grade-bound, not engagement-bound.

**Canvas Outcomes & Learning Mastery Gradebook** provide a separate, rubric-aligned mastery track with calculation methods (decaying average, n-mastery, highest, most recent) and configurable mastery thresholds. This is the right substrate for skill-tree gamification, but Canvas doesn't expose it as such.

**Canvas Badges** (formerly Badgr, now operating under Instructure's umbrella post-acquisition) is the Open Badges 2.0/3.0 issuer integration. It awards externally-portable badges backed by Credly-style backpacks. FERPA risk is the same as any Open Badge: assertions are public URLs containing identifying metadata (badge image, learner email by default, issuer info, evidence). For K-12 this is a problem; for corporate/professional this is the entire point.

**Canvas Live Events & Webhooks** is the most important API for gamification. Documented event types include `assignment.submission_created`, `assignment.submission_updated`, `assignment.quiz_submitted`, `root_account.grade_change`, `root_account.attachment_created`, `root_account.plagiarism_resubmit`, and many more (Canvas's full Caliper-format event stream covers asset_accessed, course_progress, discussion_topic_created/entry_created, login, logout, session, etc.). Delivery is via HTTPS webhook or AWS SQS queue. Live Events are documented as "well suited for analytics and data collection" and explicitly *not* for low-latency UI — Canvas does not guarantee ordering and does not deduplicate, so an in-loop gamification engine must idempotency-key on event_id and order on `metadata.event_time`.

**Canvas LTI gamification ecosystem** is shallow. There are LTI tools for badging (Accredible, Credly, Badgr Pro), some leaderboard widgets, and Mastery Connect for K-12. There is no widely-deployed Canvas-native Octalysis-style framework.

### 1.3 Brightspace / D2L Gamification

Brightspace ships a more usable native gamification kit than Canvas. The **Awards Tool** supports two types — *Badges* (Open Badges 2.0 compliant, baked with Issuer, Criteria, and Issue date metadata) and *Certificates* (PDF-generated, auto-password-protected). Both are awarded either manually by instructors or automatically via **Release Conditions**, Brightspace's general-purpose conditional logic engine (the equivalent of Canvas Mastery Paths but applicable to every tool — grades, completion, quiz scores, discussion participation, checklist items, content topic viewed, classlist enrollment, dates, etc.). Release Conditions can be combined with **Intelligent Agents** (scheduled rules that scan for matching learners and trigger actions, including badge awards and email).

The **Awards Widget** displays earned/earnable awards in a sidebar; the **Awards Leaderboard Widget** ranks learners by badges or credits earned. As of January 2024, learners can export earned credentials in Open Badges 2.0 format (a `.png` with embedded JSON metadata). D2L Lumi and the broader Brightspace analytics suite provide engagement data downstream.

Brightspace's flexibility advantage over Canvas: Release Conditions are universal across every tool and combine boolean logic across dozens of condition types; the Awards Tool is built-in (not LTI); leaderboards are first-class widgets. Disadvantages: still no XP/leveling, no streak tracking, no Octalysis-aware drives, and leaderboards have only coarse privacy controls — see Appendix A for confirmed details.

### 1.4 Octalysis Framework (Yu-kai Chou)

Yu-kai Chou's Octalysis maps all human motivation to 8 Core Drives, arranged on an octagon. Per Chou's 2015 book *Actionable Gamification* and his Octalysis Group site (used by 175+ enterprise clients including Microsoft, LEGO, Salesforce, Pfizer, Booking.com, with 3,700+ academic citations):

| #    | Core Drive                               | Type                   | Education / LMS Mechanics                                    |
| ---- | ---------------------------------------- | ---------------------- | ------------------------------------------------------------ |
| 1    | **Epic Meaning & Calling**               | White Hat, Right Brain | Mission framing ("Become the next great engineer"), beginner's luck moments, narrative arcs, cohort missions |
| 2    | **Development & Accomplishment**         | White Hat, Left Brain  | XP, points, ranks, levels, progress bars, badges, mastery meters — *Chou warns these are the most overused and least sufficient* |
| 3    | **Empowerment of Creativity & Feedback** | White Hat, Right Brain | Open-response prompts, project work, peer review, sandbox/labs, immediate auto-graded feedback loops (this is the "evergreen" drive — generates engagement without burnout) |
| 4    | **Ownership & Possession**               | White Hat, Left Brain  | Avatar customization, personal dashboards, points-as-currency, collection (badge sets), skill-tree ownership |
| 5    | **Social Influence & Relatedness**       | White Hat, Right Brain | Peer recognition, cohorts, mentor relationships, leaderboards, study groups, friend streaks |
| 6    | **Scarcity & Impatience**                | Black Hat, Left Brain  | Limited-time challenges, dripped/torchbearer content, appointment dynamics, queued cohorts |
| 7    | **Unpredictability & Curiosity**         | Black Hat, Right Brain | Mystery boxes, variable XP rewards, surprise mini-quizzes, "you might know this" prompts |
| 8    | **Loss & Avoidance**                     | Black Hat, Left Brain  | Streaks (and streak loss), rank decay, expiring points, "complete this or lose your seat" |

**Left brain drives are extrinsic/logic-based; right brain drives are intrinsic/emotion-based.** **White Hat drives feel good and empower; Black Hat drives are urgent and motivating but produce burnout if overused.** Chou's repeated recommendation for education: lead with CD3 (Creativity & Feedback) and CD1 (Epic Meaning), use CD2 to scaffold beginners, deploy CD8 (loss) sparingly. The famous critique is that 90% of LMS gamification is stuck in CD2 alone (points/badges/leaderboards) — "PBL" — which is why most LMS gamification fails after 4–6 weeks once the novelty fades.

The framework defines **four player-journey phases**: Discovery (why does someone arrive?), Onboarding (first 60 minutes — teach the rules through play), Scaffolding (the long middle — sustained engagement loops), and Endgame (what keeps the 6-month veteran engaged when there is nothing left to "achieve" for the first time?). Most LMS gamification only designs for Onboarding.

Octalysis Levels: Level 1 is single-driver tagging; Level 2 distinguishes per player type and journey phase; Level 3 introduces the "Anti Core Drives" (anti-CD2 = boredom, anti-CD8 = recklessness) and tracks emotion across time.

### 1.5 Best-in-Class Reference Platforms

**Duolingo** is the gold-standard consumer gamification system. The architecture layers **XP as a universal currency** (earned per lesson, emitted immediately on completion as a tight reward signal), **streaks** (with Streak Freeze items), **customizable daily goals**, **weekly leagues** (~30 learners per pool, promotion/demotion at week end — Duolingo reports introducing leagues increased lesson completion by 25%), **hearts/energy** (lightweight failure friction), **gems/lingots** (in-app currency for streak freezes, avatar customization), **tiered achievements**, **friend streaks** and friend quests (social layer), **monthly quests**, and **time-limited XP boosts**. See Appendix A for confirmed status of the Hearts→Energy transition as of mid-2026.

**Khan Academy** uses Energy Points (effort, not mastery) and a 5-tier badge ladder: **Meteorite → Moon → Earth → Sun → Black Hole**, plus secret Challenge Patches. Critically, Khan distinguishes Energy Points (effort) from Mastery (skill demonstrated), which is a model the target platform should copy.

**Stack Overflow** uses reputation (numerical, earned through upvotes; capped at 200/day) and a bronze/silver/gold badge system. Reputation thresholds unlock privileges. The key insight: **reputation as gated capability**, not just decoration.

**Coursera, Codecademy, LinkedIn Learning**: certificates, streaks, learning paths, skill assessments, peer learning, discussion upvotes. Less innovative than Duolingo but proven at adult/professional scale.

**Classcraft** (the original — now sunset by HMH) pioneered RPG-style classroom gamification: character classes (Warrior, Mage, Healer), HP/MP/XP, team-based protection mechanics, behavior-triggered powers. A 2021 meta-analysis found Classcraft significantly improved learning achievement (d=0.621) and motivation (d=0.608).

**Habitica** treats life tasks as RPG character progression. **Minecraft Education / Roblox Education** offer immersive, sandbox-based intrinsic CD3 engagement.

### 1.6 FERPA & Privacy Compliance

Under FERPA (20 U.S.C. § 1232g; 34 CFR Part 99), an "education record" is any record directly related to a student maintained by an educational agency or institution. **Gamification data — points earned in a graded course, badges tied to graded assessments, leaderboard standings keyed to identifiable students — is an education record** when maintained by a FERPA-covered institution and tied to PII. Disclosure to third parties requires written consent of the parent (under 18) or eligible student (18+ or in postsecondary), with narrow exceptions.

**Open Badges and FERPA:** the IMS Global Open Badges 2.0 spec embeds learner identifier (email by default) and badge metadata in publicly resolvable assertion URLs. Sharing a baked badge is functionally a disclosure of an education record. FERPA-compliant treatment: keep badges *internal* by default; require opt-in for Open Badges export; for K-12, require parental consent; default the learner identifier to a hashed/salted opaque ID rather than email.

**Leaderboard privacy:** named, ranked public leaderboards displaying student grades or grade-derived metrics are FERPA disclosure. Mitigations include opt-in, pseudonymous handles, cohort-scoped visibility, relative-only displays (deciles, percentile bands without names), or instructor-only views.

**COPPA** (15 U.S.C. § 6501) adds a separate regime for under-13 users. The FTC finalized major COPPA Rule amendments in January 2025, effective April 22, 2026 — the first significant overhaul since 2013. Key changes: expanded definition of personal information to include biometric identifiers and government-issued identifiers, stricter data minimization, and enhanced parental rights. A custom LMS targeting K-12 must support a "COPPA mode" that disables social features, public leaderboards, and external badge export for under-13 accounts.

### 1.7 xAPI / LRS Integration for Gamification

xAPI (formerly Tin Can) is the right substrate for trigger architecture. Statements are `actor + verb + object + [result] + [context]` JSON-LD records sent to a Learning Record Store. ADL's core verb registry includes `experienced`, `attempted`, `completed`, `passed`, `failed`, `mastered`, `progressed`, `answered`, `interacted`, `attended`, `voided`. The TinCanAPI verb registry adds `viewed`, `earned`, `awarded`, `bookmarked`, `commented`, `liked`, `recommended`, `shared`, and many others.

cmi5 (the xAPI profile that succeeds SCORM, adopted by US DoD) standardizes ten launch verbs and is the right import target for SCORM-legacy content. Learning Locker (open-source), SCORM Cloud, Watershed, and Yet Analytics are mature LRS options that support custom dashboards and xAPI statement forwarding.

The architectural implication for the proposed LMS: **every gamification-relevant action emits an xAPI statement**. The rule engine subscribes to the LRS (or to the internal event bus that mirrors LRS shape). External systems can plug in by subscribing to the same stream.

---

## PART 2 — Master Trigger Taxonomy

A comprehensive catalogue of every gamification trigger the platform should support. Each is expressed as a normalized event shape: `event_type | actor | object | result | context`. All triggers fan out through the rule engine and can be composed.

### A. Content & Progress Triggers

- `content.viewed` (page, video, file, scorm/cmi5 launch)
- `video.watched` (with percent-completion threshold, anti-skip detection)
- `lesson.started` / `lesson.completed`
- `module.completed` (all required items satisfied)
- `course.enrolled` / `course.started` / `course.completed`
- `assignment.opened` / `assignment.submitted` / `assignment.resubmitted`
- `assignment.submitted_on_time` / `assignment.submitted_early` / `assignment.submitted_late`
- `quiz.started` / `quiz.submitted` / `quiz.passed` / `quiz.failed`
- `quiz.passed_with_threshold` (≥80%, ≥90%, perfect)
- `quiz.improved_on_retake` (delta > N)
- `question.answered_correctly` / `question.answered_incorrectly`
- `essay.submitted_for_grading`
- `path.branched` (Mastery Path conditional item assigned)
- `learning_objective.progressed`

### B. Social Triggers

- `discussion.topic_created` / `discussion.reply_posted` / `discussion.reply_received`
- `comment.posted` / `comment.upvoted` / `comment.received_upvote`
- `peer_review.submitted` / `peer_review.received`
- `helpful_answer.marked` (instructor or peer flags a contribution as helpful)
- `study_group.joined` / `study_group.created` / `study_group.hosted_session`
- `mentor.session_completed`
- `friend.added` / `friend.streak_started`
- `nomination.received` (peer nominates user for a badge — GamiPress Nominations pattern)
- `content.shared` / `content.recommended`

### C. Mastery Triggers

- `outcome.mastered` (Canvas Outcomes mastery threshold met)
- `skill.demonstrated` (rubric criterion ≥ threshold across N artifacts)
- `competency.achieved` (Brightspace-style competency framework)
- `rubric.threshold_met` (specific row/criterion)
- `mastery.recovered` (re-mastery after decay — supports decaying-average calculation)
- `skill_tree.node_unlocked`
- `prerequisite.satisfied`

### D. Streak & Consistency Triggers

- `session.daily_login` (first action of the day, learner-local timezone)
- `streak.extended` (N=1, 3, 7, 14, 30, 100, 365 thresholds)
- `streak.broken` (negative trigger — see Section G)
- `streak.frozen` (freeze item consumed)
- `streak.repaired` (paid/earned restoration)
- `daily_goal.met` / `daily_goal.exceeded`
- `weekly_goal.met`
- `on_time_submission.consecutive` (N assignments on time in a row)
- `study_session.completed` (≥N minutes engaged)

### E. Milestone Triggers

- `account.first_login`
- `course.first_completed` / `course.tenth_completed` / `course.hundredth_completed`
- `points.threshold_reached` (1k, 10k, 100k, 1M)
- `rank.promoted` / `rank.demoted`
- `level.reached`
- `badge_set.completed` (collection complete)
- `anniversary.account` (1yr, 5yr)
- `cohort.completed_together`

### F. Instructor & Admin Triggers

- `award.manually_granted` (instructor presses a button)
- `award.manually_revoked`
- `points.bulk_assigned` (cohort-wide)
- `class.event_triggered` (instructor fires "everyone in section X gets +50 XP")
- `recognition.given` (instructor calls out specific learner publicly within cohort)
- `comment.from_instructor` (specific feedback verb)
- `live_session.attended` (synchronous class, webinar)
- `office_hours.attended`
- `exception.granted` (instructor overrides a rule)

### G. Negative / Recovery Triggers

- `streak.broken` → triggers `re-engagement.email`, `streak.repair.offered`, optional `points.partial_credit` for first-day-back
- `badge.revoked` (rare; auditing/integrity violation)
- `rank.dropped` (decaying ranks)
- `points.expired` (use-it-or-lose-it economies)
- `inactive.7_days` / `inactive.30_days` (re-engagement campaign trigger)
- `at_risk.detected` (analytics flags low-engagement learner — instructor alert)
- `quiz.failed_twice` → triggers `path.remediation_assigned`, `tutor.offered`
- `goal.missed` → optional grace mechanic, instructor notification

### H. Administrative & System Triggers

- `profile.completed` (avatar, bio, learning goals set)
- `profile.photo_uploaded`
- `beta.opted_in`
- `feedback.submitted`
- `accessibility_preference.set`
- `consent.granted` (FERPA/COPPA opt-in step — gating, not rewarding)
- `data_export.requested`
- `device.added` (mobile install)
- `notification_preference.configured`

### I. External / API Triggers

- `webhook.received` (any third-party system can POST an event)
- `lti.tool_event` (an external LTI tool reports completion)
- `xapi.statement.received` (any conformant xAPI statement)
- `canvas.live_event` (Canvas Live Events bridge — see Section 1.2)
- `zapier.recipe_fired`
- `scheduled.cron` (time-based triggers — "every Friday at 5pm, award weekend warriors")

---

## PART 3 — PRD-Style Feature Specification

### 3.1 Points Economy

**Goal:** A flexible, multi-currency system that distinguishes effort from mastery and supports both academic and engagement metrics.

**Spec:**

- **Multiple point types**, configurable per tenant. Recommended defaults:
  - **XP** (effort-based, non-spendable, drives levels and leaderboards) — Khan Academy "energy points" pattern
  - **Mastery Points** (skill-based, tied to graded outcomes, FERPA-flagged as education record)
  - **Gems** (spendable currency, earned through gameplay, used for streak freezes, hint unlocks, avatar cosmetics — never tied to grades)
  - **Reputation** (social capital — Stack Overflow pattern; gates capabilities)
- **Earn rules** are rule-engine entries: `IF event matches X AND constraints THEN award N of type Y`. Constraints include rate limiting (max once per day per source), cooldowns, role filters, course/section filters, and decay windows.
- **Anti-abuse:** all transactions logged immutably (MyCred pattern); rule-level caps; daily caps per source; instructor approval required for transactions above threshold.
- **Decay/expiration:** per-type configurable. Gems may expire after 90 days idle; XP never expires; reputation decays slowly to keep recent contribution weighted.
- **Redemption:** points can be spent on: streak freezes, hint reveals, avatar items, deadline extensions (with instructor permission), "ask the instructor a free question," course unlocks, real-world coupons (corporate/loyalty integration).
- **Transfer/gifting:** disabled by default in K-12; opt-in for higher ed; default-on for corporate.
- **Multi-tenant isolation:** point types and balances scoped to course, program, or org per tenant policy.

### 3.2 Badges & Achievements

**Goal:** Recognize discrete accomplishments with FERPA-aware default privacy, optional Open Badges export.

**Spec:**

- **Achievement types** (GamiPress pattern): tenant-defined custom post types (badge, quest, mission, stamp, certification, micro-credential).
- **Award logic** composed of one or more `requirement` steps, each linked to a trigger from the taxonomy. Sequential or any-order. Manual override always allowed for instructors with audit log.
- **Tiered families** (Khan Academy 5-tier pattern): meteorite/moon/earth/sun/black-hole; each course can define its own tier names.
- **Evidence attachment:** every award stores criteria URL, optional artifact links, and timestamp.
- **Display modes** per badge (FERPA-aware):
  - **Internal-only** (default) — visible to learner and instructor only
  - **Cohort** — visible within course
  - **Institution** — visible across the org
  - **Public/Open Badges 2.0 export** — opt-in, requires eligible-student consent, defaults OFF for K-12
- **Open Badges integration** via Badgr/Credly/Accredible LTI; baked PNG export with embedded JSON-LD; assertion endpoint configurable per tenant; learner identifier defaults to opaque ID with optional email upgrade on consent.
- **Secret badges** (CD7 Unpredictability): hidden requirements, surprise reveal animations.
- **Negative badges:** the system supports but does not default to revocable badges for integrity violations; revocation logged with reason.

### 3.3 Ranks & Levels

**Goal:** Long-arc progression visible at a glance, with meaningful unlocks.

**Spec:**

- **Ranks** = named tiers with thresholds (XP-based, points-based, or requirement-based).
- **Levels** = numeric counterpart (1-N), typically computed from XP.
- **Unlocks per rank must be meaningful** (Stack Overflow privilege pattern). Examples:
  - Rank 3: unlock peer review privilege
  - Rank 5: unlock "host a study room" capability
  - Rank 10: unlock "propose course content" capability
  - Rank 20: mentor designation
- **Rank promotion notification:** triggers email, in-app notification, optional public announcement (opt-in).
- **Rank decay/demotion:** off by default in education contexts; optionally on for corporate sales training.
- **Custom rank visualization:** images, animations, color, optional avatar frames.

### 3.4 Leaderboards

**Goal:** Drive Octalysis CD5 (Social Influence) and CD2 (Accomplishment) without violating FERPA.

**Spec:**

- **Scopes:** global, institution, course/cohort, study-group, friend-list, peer-bracket (~30 learners — Duolingo league pattern).
- **Time windows:** all-time, this-term, this-month, this-week, today, "live" (last hour).
- **Metrics:** XP, badges-earned, streaks, mastery-points, custom (instructor-defined).
- **Display modes** (FERPA-aware):
  - **Anonymous mode** (default for K-12): handles only, no real names
  - **Pseudonymous mode**: chosen display name (no PII)
  - **Cohort-named mode**: real names visible only inside the cohort
  - **Public-named mode**: requires explicit consent; opt-in flag per learner
- **Relative-only views:** "you are in the top 20%" without showing names — recommended K-12 default.
- **Promotion/demotion mechanics** (Duolingo Leagues): top-N promote at week end, bottom-M demote, middle stay. Configurable per tenant.
- **Opt-out:** every learner can opt out of all leaderboards; opting out does not reduce their XP or remove them from awards.
- **No grade-derived rankings public by default** — this is the FERPA tripwire. Effort-derived (XP from logins, attempts, time-on-task) is safer than mastery-derived.

> **Note:** Brightspace's Awards Leaderboard does NOT provide per-learner opt-out; this is confirmed as an admin-only blunt org-level control. Per-learner opt-out is a competitive differentiator — see Appendix A.

### 3.5 Streaks & Consistency Mechanics

**Goal:** CD8 Loss & Avoidance + CD2 Accomplishment, with safety rails to avoid harm.

**Spec:**

- **Tracked actions:** daily login, lesson completed, assignment submitted, study session ≥N minutes. Configurable per course.
- **Streak Freeze**: automatic + earnable. Recommend 2 auto-applied freezes per month plus earnable freezes.
- **Streak Repair:** within 48 hours of breaking, pay gems or complete a "comeback quest" to restore.
- **Customizable daily goal:** 1 XP / 10 XP / 20 XP / 50 XP — Duolingo pattern; lets users set a target they can hit on bad days.
- **Streak notifications:** in-app reminder mid-day, push notification 2 hours before midnight in learner timezone. Tone configurable (gentle vs. dramatic). **K-12 mode caps push notification frequency** and never uses guilt-inducing imagery.
- **Mercy mode:** during illness/leave, streak pauses without breaking.
- **Anti-obsession safeguard:** a "healthy streak" mode caps daily goals and surfaces wellness messaging at 100+ day streaks.

> **Design note from Appendix A:** Duolingo's Energy system (replacing Hearts as of July 2025, still rolling out mobile-only as of May 2026) depletes energy on *correct* answers — a design that severs the link between performance and progress. Your friction mechanic should deplete only on disengagement, never on correct performance.

### 3.6 Master Paths / Adaptive Unlocks

**Goal:** Canvas Mastery Paths but better — works with any trigger, not just graded source items; integrated with Outcomes; supports CD3 Creativity branching.

**Spec:**

- **Source events** can be any trigger from Part 2 (not just `quiz.passed`). Branch on streak counts, badge ownership, peer-review scores, time-of-day, role.
- **Conditional items:** any course resource — page, assignment, quiz, file, external URL, LTI tool, video, sub-module.
- **Branches:** `AND` (must complete all), `OR` (learner choice), `XOR` (system selects based on rule), `WEIGHTED` (probabilistic — supports CD7 Unpredictability).
- **Skill-tree visualization:** Duolingo-style path UI with locked/unlocked/mastered states.
- **Mastery gates:** node unlocks require either grade threshold OR outcome-mastery OR peer-validated competency OR instructor sign-off.
- **Difficulty branching:** auto-route advanced students to enrichment; struggling students to remediation. Always grade-impact-neutral.
- **Instructor preview:** "view as struggling student / proficient student / advanced student" mode.

> **Integration note from Appendix A:** Canvas Mastery Paths fires on grade-posted, not on submission. The trigger is gated by the course/assignment Grade Posting Policy. On auto-graded quizzes with Automatic posting, this fires within seconds of submission. With Manual posting, it waits for instructor action. Your Canvas Live Events bridge must listen to `grade_change`, not `quiz_submitted`. See Appendix A §A.2 for full timing analysis.

### 3.7 Social Gamification

**Goal:** CD5 Social Influence in FERPA-compliant cohort-scoped form.

**Spec:**

- **Cohorts/Guilds:** instructor-created or self-selected groups (5-30 learners). Shared dashboard, shared challenges.
- **Team mechanics (Classcraft pattern):** team HP / team XP / team buffs; one member's struggle visibly affects the team, encouraging peer help.
- **Peer recognition:** kudos, props, peer-awarded micro-badges (with cap to prevent spam).
- **Peer review queue** with reputation gating (Stack Overflow pattern) — only learners above rank N can give peer reviews on graded artifacts.
- **Friend streaks:** opt-in mutual streaks (Duolingo pattern). Disabled by default in K-12.
- **Study rooms:** scheduled or ad-hoc; attendance + duration emits triggers.
- **Mentor matchmaking:** higher-ranked learners voluntarily mentor lower-ranked.
- **Anti-bullying & harassment:** rate limiting on kudos/comments, reporting flow, instructor moderation queue, automatic muting on negative-velocity patterns.

### 3.8 Notifications & Feedback Loops

**Goal:** Tight reward signals without becoming spam.

**Spec:**

- **Channels:** in-app toast, in-app inbox, email, push (web push + mobile), SMS (opt-in only), Slack/Teams (corporate), parent email (K-12 with consent).
- **Event subscriptions:** learners choose per-event-type which channels to use.
- **Throttling:** max 3 push per day, max 1 streak-warning per day; aggregation of small events into digest if volume spikes.
- **Tone profile:** "professional," "playful," "minimal."
- **Variable reward presentation** (CD7): occasional surprise animations, surprise bonus XP, mystery chests on completion — cap frequency to ~10% of events to preserve novelty.
- **Quiet hours:** no push between 9pm–7am learner-local by default; configurable.
- **Reward latency target:** in-app reward signal ≤ 300ms after event; email within 60s; push within 30s.

### 3.9 Instructor & Admin Controls

**Goal:** Instructors must feel they own the gamification of their course.

**Spec:**

- **Recipe builder (Uncanny Automator pattern):** no-code `WHEN X THEN Y` rules. Drag-and-drop. Templates library.
- **Manual award console:** single-click award/revoke, with required reason field; cohort-wide award.
- **Bulk events:** "fire `class.field_trip_completed` for all 24 students."
- **Override switches:** disable streaks for an individual student (illness, IEP accommodation); pause gamification for a course; emergency-disable all leaderboards.
- **Cohort manager:** group learners, view group dashboards, set group challenges, name group leaderboards.
- **Curriculum-tagged rewards:** automatically award the "Algebra Apprentice" badge when learner masters all algebra outcomes.
- **Audit log:** every manual award, revocation, and override timestamped with actor and reason.
- **Recipe approval workflow** for institution admins (optional, defaults off in higher ed, on in K-12).

### 3.10 Analytics & Reporting

**Goal:** Learner self-insight + instructor early warning + admin compliance reporting.

**Spec:**

- **Learner dashboard:** XP history chart, badge gallery, current streaks, rank progression, "what's next" preview, anonymized cohort comparison.
- **Instructor dashboard:** engagement heatmap (who's engaged, who's at risk), streak distribution, badge earn distribution, leaderboard health, recipe firing volume, last-7-days activity per learner.
- **At-risk detection:** ML-lite scoring — declining streaks + missed assignments + low engagement velocity surfaces a "check on this learner" prompt.
- **Outcome correlation:** does badge X correlate with course grade? Does streak length correlate with mastery? Exportable to institutional research.
- **Admin compliance views:** FERPA-tagged data inventory; consent-status report; audit log of manual awards; per-tenant data-export & deletion.
- **xAPI statement export:** every gamification event available as xAPI statements; queryable via LRS API.
- **Caliper Analytics support** for Canvas-ecosystem compatibility.
- **Anonymized aggregate research export:** institutions can opt into contributing de-identified engagement data to a shared benchmark.

### 3.11 LTI / API / Webhook Architecture

**Goal:** Canvas-LMS-compatible interop, headless-capable, and a sane event schema.

**Spec:**

- **LTI 1.3 / Advantage** compliant — the platform installs as an LTI tool in Canvas, Brightspace, Moodle, Blackboard. Names & Roles Provisioning, Assignment & Grade Services, Deep Linking supported.

- **Canvas Live Events bridge:** out-of-the-box subscription to `submission_created`, `submission_updated`, `quiz_submitted`, `grade_change`, `course_progress`, `attachment_created`, etc. Idempotency-keyed on event_id; ordered by `metadata.event_time`. Configurable mapping of Canvas event → internal trigger.

- **Outbound webhooks:** every internal trigger and reward event emits a signed HTTPS webhook to subscribed endpoints. JWT-signed, HMAC-verifiable. Retry policy: exponential backoff, 5 attempts, dead-letter queue with admin alert.

- **REST API (headless):** every entity accessible via `/api/v1/...` with OAuth 2.0 + JWT auth.

- **xAPI/LRS endpoints:** built-in conformant LRS at `/xapi/`, or configurable forward to external LRS.

- **Recipe API:** programmatic creation, listing, enabling/disabling of trigger recipes.

- **Webhook event schema:**

  ```json
  {
    "event_id": "uuid",
    "event_type": "assignment.submitted",
    "event_time": "ISO8601",
    "actor": { "id": "opaque", "role": "learner" },
    "object": { "type": "Assignment", "id": "...", "context": { "course_id": "...", "section_id": "..." } },
    "result": { "score": 0.92, "passed": true },
    "context": { "tenant_id": "...", "policy_flags": ["ferpa_protected"] },
    "signature": "..."
  }
  ```

- **Rate limits:** 1000 req/min per API key default; configurable per tenant.

- **Sandbox tenant** for integrator development.

### 3.12 Compliance Layer

**Goal:** FERPA, COPPA, GDPR, accessibility (WCAG 2.2 AA) — not bolted on, but architectural.

**Spec:**

- **Tenant mode flag:** `K12`, `HigherEd`, `Corporate`, `Professional`. Sets defaults for leaderboard visibility, public badge sharing, push notification frequency, social features, parental access, data retention.
- **COPPA mode** (auto-applied for any K-12 tenant or any individual under-13 account): disables friend streaks, public leaderboards, Open Badges export, behavioral profiling beyond core function. Requires verifiable parental consent before any PII collection.
- **FERPA tagging:** every data field flagged as `directory_information`, `education_record`, `non_PII`, or `instructor_metadata`. Disclosure pathways enforced at the data-access layer.
- **Opt-in controls** for learners (and parents for K-12): per-feature toggle for leaderboard visibility, badge sharing, social features, friend mechanics, push notifications, behavioral analytics.
- **Data minimization:** by default, opaque IDs used in all internal records; PII materialized only at display layer for authorized viewers.
- **Right to delete:** GDPR/FERPA-aligned full-erase workflow with downstream cascade.
- **Audit log:** all disclosures logged.
- **Annual FERPA notice** template generation built in for institutions.
- **Accessibility:** every gamification surface keyboard-navigable, screen-reader-labeled, color-blind-safe palette; alternative non-animated mode; cognitive-load mode reduces simultaneous reward signals.
- **Bias & fairness review** for any ML-based at-risk scoring before launch.

---

## PART 4 — Competitive Gap Analysis

The most engaging LMS on the planet does not exist today. Here is the whitespace.

**1. Octalysis-aware design across all 8 drives, not just CD2.** Every existing LMS gamification system lives in CD2 (Development & Accomplishment) — points, badges, leaderboards. An LMS that purposefully scaffolds CD1 (cohort missions, year-long narratives), CD3 (immediate auto-feedback, learner-built content, peer-reviewed creative submissions), and CD4 (avatar identity, owned skill trees) would feel categorically different. Lead with right-brain drives; use CD2 as scaffolding for newcomers, not the substance.

**2. Trigger-first composability that instructors actually control.** Every existing system either hard-codes triggers (Canvas Mastery Paths is grade-only) or hides them behind admin-only configuration (GamiPress, MyCred). A recipe builder UI for instructors — "WHEN any trigger from a 200-event taxonomy THEN any reward from a composable catalog" — is whitespace.

**3. FERPA-native, not FERPA-retrofitted.** Every LMS gamification feature ships with privacy as a configuration. Make it the schema. The platform should be the first to ship with a per-field FERPA-classification taxonomy enforced at the API layer, a built-in COPPA mode, and a default-safe K-12 posture. Confirmed as a moat against Canvas Badges (Open Badges-by-default) and against all WordPress plugins.

**4. Streaks and loss-aversion mechanics calibrated for learning, not consumer apps.** Duolingo's streak design is masterful but ruthless. The platform should offer Duolingo-grade mechanics with deliberate safety rails: customizable daily goals, automatic mercy mode during illness/IEP accommodation, anti-obsession warnings, instructor-visible streak-stress signals.

**5. Mastery vs. Effort as separate currencies.** Khan Academy nailed this; almost no one else does. Mastery points are an education record (FERPA); effort XP is not. Separating them lets you put XP on a public leaderboard while keeping Mastery private — a privacy win and a pedagogical win.

**6. Skill trees instead of linear modules.** A genuine skill-tree UI — visual, learner-explorable, unlockable, with prerequisites and parallel paths — is missing across all major LMSes.

**7. Per-learner leaderboard opt-out.** Confirmed missing from Brightspace (admin-only masking added June 2025, no individual toggle). Missing from Canvas. Build it as a first-class learner control.

**8. Native xAPI/cmi5 as the spine, not as an export.** Most LMSes treat xAPI as a reporting destination. Designing the platform with xAPI statements as the *internal* event substrate makes every feature portable, every integration plug-and-play, and federal compliance trivial.

**9. Reputation as gated capability (Stack Overflow pattern).** No LMS today unlocks real platform powers based on engagement. Learners at Rank 5 can host study rooms; Rank 10 can peer-review; Rank 15 can propose course-content tweaks; Rank 20 get mentor designation. This is a *behavior* differentiator, not just a *display* differentiator.

**10. Endgame design.** After a learner has earned every badge and reached every rank, what's left? Prestige resets, legacy mentor roles, cohort-leadership privileges, content-authorship tools, alumni networks. This turns a 1-year L&D program into a 10-year community.

---

## Recommendations

**Immediate (next 30 days):**

1. Stand up the **event-bus architecture** first. Implement xAPI statement emission for 20–30 core triggers. Spec the canonical event envelope (Section 3.11).

2. Implement a **Canvas Live Events ingestion bridge** as the first integration target. Map Canvas `grade_change` (not `quiz_submitted`) → internal triggers. Use SQS for production-grade ordering and durability.

3. Build the **tenant-mode flag** (K12/HigherEd/Corporate/Professional) and the FERPA field-classification taxonomy *before* any UI work.

   **Short term (Q1):**

4. Ship **points (XP + Mastery + Gems), badges (internal-only default), and the recipe builder** as the MVP feature set. Lead with Octalysis CD3 (immediate feedback) and CD5 (cohort recognition); avoid leaderboards in v1 entirely to dodge FERPA exposure during testing.

5. Build a **read-only learner dashboard** and an **instructor recipe library** with 50 pre-built templates.

   **Medium term (Q2–Q3):**

6. Add **streaks with safety rails**, **ranks with capability unlocks** (Stack Overflow pattern), and **cohort-scoped leaderboards** with per-learner opt-out.

7. Skill-tree visualization and Mastery Paths-equivalent (working with any trigger, not just grades).

8. Open Badges 2.0/3.0 LTI integration with Badgr/Credly/Accredible — opt-in only; OB 3.0 DID-based earner identifier preferred over email for privacy.

   **Longer term (Q4+):**

9. ML-light at-risk detection, learner self-insight, instructor early warning.

10. Endgame mechanics: prestige, mentor designations, alumni community.

11. Marketplace for community-contributed recipes and badge templates.

---

## Caveats

- **Canvas Live Events documentation** explicitly warns that the service is "not for applications that need their data immediately and as up-to-date as possible." For latency-sensitive gamification (streak warnings, instant XP feedback), supplement Live Events with direct API polling or expect some delay.
- The Octalysis-to-LMS-mechanic mapping in this brief is the author's synthesis; a canonical "Octalysis for LMS" mapping does not exist and instructional designers should iterate against their own learner population.
- **MyCred and GamiPress feature parity** is a moving target — both ship updates monthly. Treat feature lists as snapshots.
- **Classcraft** as a vendor was sunset under HMH ownership. The Classcraft *design pattern* (RPG team mechanics, HP/MP/XP, party-of-five) is the durable reference; spiritual successors (TeachQuest, Kiwibee) are early-stage.
- The cited **Duolingo retention statistics** (25% lesson-completion lift from leagues, etc.) are from product-marketing materials; underlying methodology is not fully disclosed. Treat magnitude as directionally correct, not precisely measured causal effects.
- **FERPA application to gamification data** is well-established for grade-derived metrics; for behavioral/effort metrics (XP from logins, time-on-task) the legal posture is less settled and varies by institutional interpretation. Review with institution's general counsel before launch.

---

## Appendix A — Cross-Check Research: Five Thin Areas Verified

*Conducted as a follow-up research pass to confirm or correct the thinnest findings in the original report. All findings current as of May 2026.*

---

### A.1 Brightspace Awards Leaderboard — Does the Widget Support Student Opt-Out?

**Verdict: No learner opt-out exists. Admin-only, blunt, OFF by default.**

The June 2025 Brightspace release added `d2l.Custom.LCSWidgets.AwardsLeaderboard.MaskUsernames` — an org-wide or course-wide configuration variable that can mask student names and profile images in the Awards Leaderboard widget. Key details:

- Control is held entirely by administrators, not learners.

- The masking configuration is **OFF by default**; institutions must explicitly enable it.

- Users with the "Awards Leaderboard > See Masked User Details" permission can view real names even when masking is on — meaning institutional staff always see through the mask.

- Regardless of any configuration, learners always see their own name and profile image.

- The same June 2025 release also added `d2l.Custom.LCSWidgets.AwardsLeaderboard.ForceSortBy`, an org-level override for sorting behavior (Awards, Credits, or Not Set).

  There is no per-learner toggle, no "hide me from the leaderboard" option, and no opt-out mechanism surfaced to students in any current Brightspace documentation.

  **Version history context:** Before June 2025, even the admin-level masking option did not exist. Pre-June 2025 Brightspace tenants had only a coarse role-based control: if a learner's role was restricted from showing first/last names org-wide, they appeared as "Anonymous User" on the leaderboard — not a per-student leaderboard opt-out, but a blunt identity-suppression toggle applied across all Brightspace tools. The Awards Leaderboard widget itself is gated behind D2L's "Course Adventure Pack" / Engagement Plus license tier; institutions that haven't purchased this add-on simply don't have the widget.

  **FERPA/AADC audit risk:** Named leaderboard rankings keyed to academic performance are a peer-visible academic indicator. In some UK/EU jurisdictions, Age-Appropriate Design Code (AADC/Children's Code) analysis may additionally flag a forced-visible leaderboard as incompatible with "best interests of the child" requirements. Absent a per-student opt-out, a FERPA or AADC audit could require removing the widget entirely.

  **Design implication:** Per-learner opt-out is a genuine whitespace feature. Brightspace's approach treats leaderboard privacy as an institutional compliance checkbox, not a learner autonomy right. Building per-learner opt-out as a first-class feature — with no reduction in XP or awards for opting out — is both FERPA-safer and a meaningful UX differentiator.

---

### A.2 Canvas Mastery Paths — Score-Band Evaluation Timing

**Verdict: Fires on grade-posted, not on submission. Further gated by Grade Posting Policy.**

Multiple authoritative Canvas sources confirm the timing chain explicitly:

- The Mastery Path score-band evaluation fires after a grade is **posted** to the gradebook, not at the moment of submission.

- The precise trigger is the Canvas `grade_change` event (or the gradebook write that precedes it), not `quiz_submitted` or `submission_created`.

- Whether the grade posts immediately or requires instructor action depends entirely on the **Grade Posting Policy** applied to the assignment:

  - **Automatic Policy** (Canvas default for new courses): grades write immediately when Canvas auto-grades (e.g., a fully objective quiz). Mastery Path evaluates within seconds of the student submitting, effectively making this feel like a submission trigger. However, if the quiz contains any manually-graded questions, the grade is partial until the instructor grades and the final score triggers the path.

  - **Manual Policy**: grades are held from students — and from Mastery Path evaluation — until the instructor explicitly clicks "Post Grades." This could be hours or days after submission.

    **The FERPA-grade-posting-policy intersection:** If a course uses manual posting to protect grades before release (a common practice to prevent early students from sharing answers), Mastery Path branching is also blocked. This is a non-obvious UX problem for instructors who assume adaptive branching is instant.

    **Canvas Live Events implication for integration:** Do not listen to `quiz_submitted` alone as the trigger for Mastery Path equivalent behavior. Listen to `grade_change`. Implement idempotency on `event_id` and sort on `metadata.event_time` (Canvas does not guarantee delivery order or exactly-once delivery). For your own LMS's adaptive path engine, fire the branching trigger on **grade-write** (when the score is finalized and visible), not on submission, and expose the distinction clearly in recipe configuration so instructors understand the latency model.

    **New Quizzes caveat:** New Quizzes (Canvas's replacement engine for Classic Quizzes) has known asynchrony issues with Mastery Paths — the instructor-side "Mastery Paths breakdown" UI that shows which score range each student landed in is confirmed missing or broken in some New Quizzes contexts per Instructure Community reports. For any workflow where Mastery Paths triggering is mission-critical, prefer Classic Quizzes for now. Note however that Instructure has signaled long-term deprecation of Classic Quizzes in favor of New Quizzes; monitor the parity gap and expect to migrate once Instructure closes it.

    **Resubmission/regrade re-fire:** Canvas explicitly warns that if a source assignment is regraded after a student has already been assigned a path, and the regrade moves the student into a different score band, they will be reassigned to the new conditional items. This means the `grade_change` trigger can re-fire for the same learner on the same assignment. Your rule engine must handle this idempotently and support the concept of "path reassignment" — not just initial assignment.

---

### A.3 Open Badges 3.0 Wallet UX for Under-13

**Verdict: No production-grade under-13 consent flow exists anywhere in the OB 3.0 ecosystem. The wallet infrastructure is still maturing for adults. Internal-only badges for under-13 are the only legally defensible default.**

The 1EdTech Open Badges 3.0 / CLR 2.0 specification is entirely silent on age gating, parental consent, and COPPA flows. It is built on W3C Verifiable Credentials and is age-agnostic; COPPA handling is left entirely to implementers. Major commercial wallets resolve this by simply prohibiting direct under-13 accounts:

- **Credly (by Pearson):** Credly's official FAQ states it "complies with COPPA by obtaining consent through K-12 institutional customers, honoring parental requests for data deletion, and implementing appropriate data privacy and security safeguards" and supports "badging for high school students who are 13 or older." There is no parent-facing consent dialog in the earner flow — the K-12 school is treated as the consenting party on behalf of parents, an arrangement permitted by the FTC's COPPA School Authorization model.

- **Canvas Badges / Badgr (Instructure / Parchment):** The Canvas Badges Terms of Service state directly: "you must be 13 years old or older to use Canvas Badges." The Privacy Policy reiterates: "Parchment Digital Badges are not directed to children under 13. Our Terms prohibit anyone under the age of 13 from using Parchment Digital Badges." (Parchment is the Instructure-owned successor brand to Badgr.) Instructure Community confirms K-12 use is supported only for students ≥13.

  **What the actual under-13 UX looks like in production:** The school issues the badge in its LMS. The badge artifact (PNG/SVG with embedded JSON) is stored server-side under the school's account, which is covered by school-mediated COPPA consent. The badge is surfaced inside the LMS or delivered to the family on request. If a family wants the badge in a personal wallet, the *parent* must create the wallet account and accept the badge on the child's behalf — none of the major wallets expose a built-in parental-consent flow for child sub-accounts; they simply bar under-13 accounts entirely.

  **If you want a parent-facing wallet for under-13:** You would need to build the verifiable parental consent (VPC) flow yourself using one of the FTC-approved methods: credit-card micro-charge, government-ID verification, knowledge-based authentication, or signed consent form. Budget 6–12 months of legal/UX work; this is non-trivial and the OB 3.0 spec gives no scaffold for it.

  Open Badges 3.0 is a significant evolution from 2.0: it adopts W3C Verifiable Credentials, supports Decentralized Identifiers (DIDs), and enables badge storage in digital wallets alongside government IDs and other credentials. Badge recipients gain cryptographic portability and self-sovereignty over their credentials. However:

- **Wallet services are still early-stage.** The Open Badge Factory team noted as of late 2025 that "there are still many open questions related to ecosystem functionality and especially the development of wallet services, which involves not only technical development but also policy considerations that are still evolving."

- **The design target for OB 3.0 wallets is adult professionals** — licensed professionals proving qualifications, job seekers presenting skill badges to employers, employers co-signing skills.

- **Regulatory exposure is heightened.** The FTC finalized COPPA Rule amendments on January 16, 2025, effective April 22, 2026. Key changes relevant to Open Badges: expanded definition of personal information now includes biometric identifiers and government-issued identifiers; any persistent identifier (including a DID tied to a child's account) is personal information; and all data collection from under-13 users requires verifiable parental consent with documented logging.

- **OB 3.0 DID-based identifiers are better than OB 2.0 email-based identifiers for privacy**, because DIDs can be opaque strings with no PII embedded. However, the claim chain (DID → badge assertion → issuer metadata) may still be linkable to a specific child if institutional records are subpoenaed or disclosed.

- **COPPA 2.0 watch:** The U.S. Senate passed COPPA 2.0 unanimously on March 5, 2026; it remains pending in the House as of this report's date. If enacted, it would extend protections to minors under 17 — which would materially change wallet/leaderboard/social-feature design assumptions for the entire teen population, not just under-13.

  **Design implication:** The architecture decision in the main report — internal-only badges as the default, with opt-in Open Badges export requiring consent — is not a limitation but the *only legally defensible posture* for any K-12 deployment under current US law. When implementing OB 3.0 export for eligible students (18+ or parental consent obtained), use DID-based earner identifiers rather than email. Build the consent checkpoint as a hard gate in the export flow (not a soft warning), and log the consent event as an audit record. If COPPA 2.0 passes, revisit age thresholds across all social and gamification features.

---

### A.4 Duolingo Energy System — Status as of Mid-2026

**Verdict: Not reversed. Hearts replaced by Energy on mobile as of May 2026. Still in gradual rollout/A/B testing. Heavy criticism but no rollback.**

The full timeline:

- **April/May 2025:** Duolingo began testing Energy with a small subset of mobile users.

- **July 3, 2025:** Broader A/B testing officially began.

- **July 2025:** Duolingo published a blog post explaining the system. Energy works as a battery: users start with 25 units/day; each exercise costs 1 unit regardless of correctness (correct answers in streaks return small amounts of energy); free users eventually deplete and must watch ads, spend gems, or subscribe to continue.

- **October–November 2025:** Rolling out widely on iOS and Android. Web app retained the old Hearts system throughout this period.

- **May 2026:** Hearts have been "partially discontinued and replaced with energy on mobile" per the Duolingo Wiki, while the web client still uses Hearts.

  Community reaction has been strongly negative. The core criticism: Energy depletes on correct answers, which means even perfect performance eventually blocks free learners — a fundamentally different (and worse) motivational model than Hearts, which only punished errors. Users describe it as a monetization mechanism that restricts learning rather than supports it. Class Central's reporting on the Q4 2025 earnings call noted that Q2 2025 DAU growth came in at the low end of Duolingo's guidance, with Energy cited as one of three contributing factors — described as perceived as "a greedy monetization move, leading to more uninstallations." CEO Luis von Ahn called 2026 an "investment year" focused on reducing friction, but Energy itself has not been pulled. Competitors Babbel and LingQ are explicitly marketing a "no daily limit on free learning" positioning to capture migrating Duolingo users per Q1 2026 user-migration reports.

  **No opt-out:** Notably, even upgrading to a Super or Max subscription does not give users a way to revert to the Hearts system — the Duolingo Wiki explicitly states: "Unlike with the hearts system, unlimited energy through a Super or Max subscription cannot be turned off by the user." The Energy system is the only available UX for mobile learners as of May 2026, regardless of subscription tier.

  **Octalysis analysis:** The Energy system damages CD3 (Creativity & Feedback) by severing the link between correct performance and continued access. It weaponizes CD8 (Loss & Avoidance) against the platform's own mission. This is precisely the "Black Hat CD8 overuse" pattern Chou warns against — it generates short-term conversion pressure but erodes long-term intrinsic motivation.

  **Design implication for your platform:** Duolingo's Energy transition is a cautionary reference, not a design inspiration. If you implement any failure-friction mechanic, it should deplete only on disengagement, idle time, or errors — never on correct performance. The Hearts model (error-penalty only) was better pedagogy than Energy (always-depleting). "No daily cap on free learning" is now the most defensible differentiator in the language-learning and LMS-gamification market.

---

### A.5 NWEA MAP / IRT — Psychometric Model and Patent Status

**Verdict: NWEA MAP uses the Rasch model (1-parameter logistic IRT). The Rasch model is 1950s academic mathematics — no patent, no licensing requirement. NWEA's proprietary assets are the RIT scale, item bank, and calibration data, not the algorithm.**

**The model:** NWEA uses the **Rasch model** (also called 1PL IRT — one-parameter logistic) to create its RIT (Rasch unIT) scales. Per NWEA's own procurement documents and technical reports: "NWEA uses the Rasch item response theory model to create its RIT scales. MAP Growth results, reported as RIT scores, relate directly to the curriculum scale in each subject area." The Rasch model was developed by Danish mathematician Georg Rasch in the 1950s and published in *Probabilistic Models for Some Intelligence and Attainment Tests* (1960). It is pure academic mathematics and is part of the published scientific literature.

**Patent status:** The Rasch model itself cannot be and has never been patented. The mathematical formula describing item response probability as a function of ability and item difficulty is in the public domain. Numerous open-source implementations exist: the `R` packages `TAM`, `eRm`, `mirt`, and `ltm` all implement Rasch/IRT models under open licenses. Python equivalents exist as well.

**What IS proprietary to NWEA:**

- The **RIT scale calibration** — the specific numerical anchor values that make RIT scores comparable across grades, years, and item sets

- The **item bank** — hundreds of thousands of calibrated items with established difficulty parameters

- The **normative data** — grade-level norms and growth norms built from millions of student administrations

- The **MAP Growth platform** — the delivery engine, reporting dashboards, and institutional data infrastructure

  Using the Rasch model or any IRT algorithm in a custom adaptive engine is legally clean. Using NWEA's item content, RIT scale values, or normative data without a license is not.

  **NWEA's proprietary innovations — what is specifically encumbered:** The MAP Growth Technical Report (2024–2025) references two NWEA-developed innovations layered on top of the public Rasch math:

- The **Enhanced Item Selection Algorithm** — NWEA's update to the standard maximum-Fisher-information item-selection approach, incorporating additional constraints beyond canonical CAT methods.

- The **Content Proximity** item-selection methodology — introduced operationally after a Spring 2022 pilot, governing how items are selected relative to previously-administered content (e.g., avoiding topically adjacent items in sequence). Both are documented in NWEA's research archive as NWEA-developed innovations and appear to be proprietary IP.

  **HMH acquisition and patent assignment:** NWEA was acquired by HMH Education Company (Houghton Mifflin Harcourt) in early 2024. The 2024–2025 Technical Report bears HMH copyright. Patents previously held by Northwest Evaluation Association may now be assigned to HMH. A freedom-to-operate search should query both "NWEA" and "HMH Education Company" / "Houghton Mifflin Harcourt" as assignees on USPTO.

  **Trademarks:** NWEA®, MAP®, MAP Growth™, and "RIT" are registered/claimed trademarks. A competitor must not use these names in product branding, but can build a Rasch-based equal-interval scale under any original name.

  **Three-stage implementation roadmap for a competing adaptive engine:**

- **Stage 1 — Algorithm (unencumbered):** Build on Rasch (1PL) with standard maximum-Fisher-information item selection and field-test embedding. This stage is fully clear of IP concern.

- **Stage 2 — Pre-launch FTO:** Commission a USPTO patent search on NWEA / HMH assignees, specifically around "item selection," "content proximity," "blueprint constraints in CAT," and "off-grade item selection." Budget $5K–$15K for a patent attorney freedom-to-operate opinion before any commercial launch.

- **Stage 3 — Naming/branding:** Coin your own scale name. Avoid "MAP," "MAP Growth," "RIT," and "Rasch Unit" as product or score terminology.

  **IRT model selection guidance for your adaptive engine:**

| Model       | Parameters                             | Item Bank Size Needed   | Best For                                  |
| ----------- | -------------------------------------- | ----------------------- | ----------------------------------------- |
| Rasch / 1PL | Difficulty only                        | ~100 responses/item     | Fastest to calibrate; start here          |
| 2PL         | Difficulty + Discrimination            | ~200–300 responses/item | Better accuracy once you have data volume |
| 3PL         | Difficulty + Discrimination + Guessing | ~500 responses/item     | MC-heavy high-stakes assessments          |

**Recommended approach:** Start with Rasch/1PL. Use **field-test item embedding** — NWEA's own calibration technique — where a fixed number of unscored "field test" slots are embedded in every learner's assessment session to gather calibration data without burdening the learner. This gives you a representative calibration sample without a separate field-test phase. Once you have 100+ responses per item, move items from field-test to operational. Build toward 2PL once response volume supports it.

**Open-source resources:**

- `catR` R package — full CAT simulation and administration with Rasch through 4PL

- `mirt` R package — multidimensional IRT including Rasch

- `py-irt` (Python) — IRT models including Rasch for educational applications

- IACAT.org — the International Association for Computerized Adaptive Testing maintains a software registry

  **No psychometrics PhD required to start** with Rasch on a closed-item bank, but one is strongly recommended before launching high-stakes adaptive assessments at scale. For low-stakes formative gamification contexts (skill-tree unlocking, adaptive content routing based on quiz performance), Rasch is well within reach of a strong engineering team using the above libraries.

---

### A.6 Actionable Recommendations Per Finding

The following thresholds and actions apply across all five verified areas. Each includes a "threshold to revisit" so the document stays durable rather than going stale.

**Brightspace leaderboard (A.1):**

- If your institution uses the Awards Leaderboard and student privacy matters, upgrade to the June 2025 release or later and enable `d2l.Custom.LCSWidgets.AwardsLeaderboard.MaskUsernames` at the org-config level.

- If FERPA/GDPR/AADC compliance is at stake, remove the Awards Leaderboard widget from course homepages entirely and rely on the private "My Awards" view per student. There is no per-student opt-out, and absent one the leaderboard may not survive a privacy audit in UK/EU jurisdictions.

- *Revisit if:* A future Brightspace release adds a per-student "do not show me on the leaderboard" toggle.

  **Canvas Mastery Paths trigger timing (A.2):**

- Build your trigger listener on Canvas Live Events `grade_change`, NOT `submission_created`. For auto-graded quizzes under Automatic posting, these fire near-simultaneously; under Manual posting, `grade_change` will not fire until the instructor posts.

- If your workflow needs guaranteed instant triggering, document a course-design requirement that the source assignment must use Automatic posting. Surface a warning in the instructor setup UI if they switch to Manual.

- For Mastery Paths specifically: prefer Classic Quizzes over New Quizzes until Instructure closes the parity gap on the breakdown UI.

- *Revisit if:* Instructure ships a "trigger on submission regardless of post policy" option, or closes the New Quizzes / Mastery Paths parity gap.

  **Open Badges 3.0 under-13 (A.3):**

- Mirror the industry consensus: do not let under-13 learners hold their own wallet account. Have the school be the COPPA-consenting party under the FTC's school authorization model; store badges server-side under the school's account; deliver badges to parents on request.

- If you want a parent-facing wallet, build the verifiable parental consent flow yourself using one of the FTC-approved VPC methods. Budget 6–12 months of legal/UX work.

- *Revisit if:* 1EdTech publishes a normative profile for child accounts; COPPA 2.0 is enacted (extends protections to under-17); or a major commercial wallet ships a built-in parental-consent sub-account flow.

  **Duolingo Energy (A.4):**

- Treat Energy as a deployed monetization mechanic with significant backlash but no rollback. Do not build product assumptions on it being reversed.

- If your product is positioned as a Duolingo alternative, "no daily limit on free learning" is the strongest available differentiator.

- *Revisit if:* Duolingo's Q1 or Q2 2026 earnings call announces an Energy modification or rollback; or if DAU growth resumes toward the stated 26% target for 2027 (suggesting users accepted Energy).

  **NWEA MAP adaptive engine (A.5):**

- Build on Rasch (1PL) with standard maximum-Fisher-information item selection. This is fully unencumbered.

- Commission a USPTO FTO search before launch targeting NWEA / HMH assignees. Budget $5K–$15K.

- Avoid "MAP," "RIT," "Rasch Unit" as product names or score scale terminology.

- *Revisit if:* NWEA/HMH publishes a new patent on its adaptive engine (searchable on Google Patents and USPTO).

---

### A.7 Caveats on This Appendix

- **Patent inventory for NWEA (A.5):** The Enhanced Item Selection Algorithm and Content Proximity innovations are confirmed as NWEA-proprietary in their technical literature, but specific USPTO patent numbers were not retrieved in this research pass. Treat the patent section as directionally correct; commission a formal FTO search before commercial deployment of any algorithm closely mirroring these approaches.
- **Brightspace (A.1):** Cannot fully rule out an obscure combination of D2L's "User Information Privacy" framework settings that effectively hides a single student from the leaderboard — D2L's privacy framework is complex. But there is no documented per-student leaderboard opt-out in any official source examined.
- **Canvas Mastery Paths (A.2):** The New Quizzes / Mastery Paths asynchrony issue is confirmed in Instructure Community reports but the exact scope of the breakdown varies by institution configuration. Classic Quizzes is the safer choice for now; long-term, New Quizzes is the durable platform choice once Instructure achieves parity.
- **OB 3.0 under-13 (A.3):** Findings are based on major U.S.-based commercial wallets. Smaller open-source / self-hosted OB 3.0 wallets (e.g., DCC-issued credentials, EUDI-aligned European wallets) may have different age-handling; those were out of scope.
- **Duolingo Energy (A.4):** Energy mechanics vary across A/B segments — gem prices, ad-refill amounts, and bonus rules differ by user group. The "25 units daily" figure is the most common configuration, not universal.
- **COPPA 2.0 (cross-cuts A.3, A.4, and the compliance layer throughout):** The U.S. Senate passed COPPA 2.0 unanimously on March 5, 2026; it remains pending in the House as of this report's date. If enacted, it would shift the relevant age threshold to under-17 and would materially affect wallet design, leaderboard opt-out logic, social features, and the Energy-mechanic comparison for any platform serving teen users. All four product-facing items above should be revisited if/when COPPA 2.0 becomes law.

---

*End of Appendix A. Last updated: May 2026.*
