# Native Gamification, Adaptive Paths & Engagement Mechanics in Major LMSes

*Claude research agent, 2026-05-12. Focus: Canvas, Brightspace/D2L, Moodle, Schoology, Khan Academy, Google Classroom.*

A research brief for Paper LMS. Goal: catalog what Canvas, Brightspace/D2L, Moodle, Schoology, Khan Academy, and Google Classroom ship natively, then distill the atomic primitives Paper LMS needs to match or exceed all of them.

---

## 1. Canvas Mastery Paths (Instructure)

Canvas Mastery Paths is a conditional-release mechanism layered onto **Modules** and built on top of the **Differentiated Assignments** infrastructure. It is the closest thing Canvas has to adaptive branching.

**Mechanics.** Inside a Module, any graded assignment, graded discussion, or graded quiz can be designated a **source item**. The teacher defines **exactly three scoring ranges** on that source (the top/bottom are pinned to the assignment's point total; the two interior cut points are editable — e.g., 0–40 / 40–70 / 70–100). Each range maps to one or more **conditional items** drawn from items already in the module. After the source is scored (manually or auto), Canvas assigns each student the items belonging to their score band. Practice quizzes, ungraded surveys, and external-tool assignments cannot be conditional items. ([Instructure Community — How do I use Mastery Paths in course modules?](https://community.canvaslms.com/t5/Instructor-Guide/How-do-I-use-Mastery-Paths-in-course-modules/ta-p/906), [Instructure: A Quick Guide to Creating Mastery Paths](https://www.instructure.com/resources/blog/quick-guide-creating-mastery-paths))

**Data model.** Mastery Paths is implemented as *automatic assignment overrides*: when a student lands in a range, Canvas creates an override for each conditional item that adds that student as an assignee. The `assignment_visibility` and `overrides` fields in the [Assignments API](https://developerdocs.instructure.com/services/canvas/resources/assignments) become populated dynamically. This is why Mastery Paths "just works" with Modules, the gradebook, and due-date logic — every assignment already knows how to be visible to "everyone, sections, groups, or specific students."

**Interaction with the rest of Canvas.**
- **Modules**: only a module item can be a conditional item.
- **Assignment Groups**: orthogonal — Mastery Paths doesn't gate by group, only by score.
- **MasteryConnect / Outcomes**: separate product. Outcomes feed the Learning Mastery Gradebook (below), not Mastery Paths directly. There is no native primitive "if outcome X is Mastered, release Y" — you must use the score on an outcome-aligned assignment as the gate.
- **Differentiated Assignments**: the substrate. Mastery Paths is essentially auto-generated overrides.

**Teacher UX.** A modal on the source item: drag two cut points, then drag-and-drop other module items into each range. No scripting. Reporting: per-assignment "Show Mastery Paths" view shows which path each student is on.

**Open-source.** Yes — Canvas LMS is AGPLv3 ([instructure/canvas-lms](https://github.com/instructure/canvas-lms)). The `conditional_release` plugin / API endpoints (`/api/v1/courses/:id/mastery_paths/…`) are inspectable, though self-hosters have repeatedly hit configuration issues ([issue #1320](https://github.com/instructure/canvas-lms/issues/1320), [#1694](https://github.com/instructure/canvas-lms/issues/1694)).

**Limit.** Three bands only. No AND/OR, no branching DAG, no "completed N of M discussions" gates, no time-based releases, no badge effects.

---

## 2. Canvas Outcomes & Learning Mastery Gradebook

Separate from Mastery Paths but architecturally important: Canvas Outcomes have **five calculation methods** that roll repeated rubric/quiz hits into a single mastery score per outcome per student ([Instructure: New and Updated Proficiency Calculations](https://community.canvaslms.com/t5/The-Product-Blog/Canvas-Outcomes-New-and-Updated-Proficiency-Calculations-Coming/ba-p/579866)):

1. **Decaying Average** — recent score gets weight w (default 65%, configurable 50–99%); prior aggregate gets 1−w.
2. **n Number of Times** — mastery must be hit n times (1–10) before it counts as Mastered.
3. **Most Recent Score**
4. **Highest Score**
5. **Weighted Average** (newer addition)

The **Learning Mastery Gradebook** is a parallel grid of students × outcomes showing each calculated proficiency. Critically, **mastery percentage is not currently a trigger source** for Mastery Paths — but it should be one in Paper LMS (this is a clear gap to exceed Canvas on).

---

## 3. Brightspace / D2L — The Gold Standard

D2L's **Release Conditions** + **Intelligent Agents** + **Awards** trio is by far the most expressive adaptive/gamification system in mainstream LMSes. If Paper LMS copies one model, copy this one.

### 3a. Release Conditions

Release Conditions attach to any of: **Content modules/topics, Discussion forums/topics, Assignments (dropboxes), Quizzes, Surveys, Grade items/categories, Checklists, Announcements, Custom widgets, Intelligent Agents, and Awards** ([D2L: About release conditions](https://community.d2l.com/brightspace/kb/articles/3432-about-release-conditions)). If multiple conditions are attached, the teacher picks **All must be met** (AND) or **Any must be met** (OR) at the rule level.

The [Valence developer API reference](https://docs.valence.desire2learn.com/res/releaseconditions.html) enumerates the atomic condition types. Below is the canonical list, grouped:

- **Awards**: `EarnsAward`
- **Checklist**: `CompletesChecklist`, `CompletesChecklistItem`, `NotCompletedChecklist`, `NotCompletedChecklistItem`
- **Classlist / enrollment**: `DaysEnrolledInCurrentOrgUnit` (time-since-enrollment gate, great for rolling enrollment), `EnrolledInGroup`, `EnrolledInOrgUnit`, `EnrolledInSection`, `RoleInCurrentOrgUnit`
- **Content**: `VisitsContentTopic`, `CompletesContentTopic`, `VisitsAllContentTopics`, `NotVisitedContentTopic`, `NotCompletedContentTopic`
- **Discussions**: `AuthorsPostsInTopic`, `NotAuthoredPostsInTopic` (plus score-on-rubric variants in newer versions)
- **Dropboxes (Assignments)**: `SubmitsToDropbox`, `NotSubmittedToDropbox`, `ReceivesFeedback`
- **Grades**: `ReceivesScoreOnGradeItem` (with min/max threshold), `NotReceivedScoreOnGradeItem`, `ReleasedFinalGrade`
- **Quizzes**: `SubmitsQuizAttempt`, `NotSubmittedQuizAttempt`, `ReceivesScoreOnQuiz`
- **Competencies / Learning Objectives**: `CompetencyAchieved`, `LearningObjectiveAchieved` / `LearningObjectiveNotYetAchieved` (mentioned in [Brightspace Knowledge Base](https://sites.google.com/nicc.edu/brightspaceknowledgebase/instructor-tutorials/instructor-quick-reference-guide/release-conditions))
- **Surveys**: completion-based
- **ePortfolio**: linked-evidence conditions

That's ~30+ atomic condition types, each parameterized (e.g., score thresholds, specific group IDs). **This is the gold-standard "atomic primitive" set** for any LMS that wants expressive adaptive paths.

### 3b. Intelligent Agents

Intelligent Agents are **scheduled rule engines**: cron-like evaluation of release conditions + activity criteria (login-since, course-activity-since), with **effects = email to student/teacher/auditor, optional automated reply, and connection to Awards** ([D2L: About Intelligent Agents](https://community.d2l.com/brightspace/kb/articles/3499-about-intelligent-agents), [Set up Intelligent Agents](https://community.d2l.com/brightspace/kb/articles/3496-set-up-intelligent-agents)). They run on a schedule (daily/weekly/etc.) OR on a one-shot/manual trigger. The trio of criteria they evaluate is **(1) login activity, (2) course access activity, (3) release conditions** — meaning Agents reuse the Release Conditions vocabulary as their predicate language. This is the architectural insight Paper LMS should steal: **the predicate language is shared between content-gating, agent-firing, and award-issuing.**

### 3c. Awards (Badges + Certificates)

Awards = **Badges** (Open Badges 2.0 format, displayable externally) + **Certificates** (auto-generated, password-protected PDFs) ([Brightspace: Awards Tool](https://d2lhelp.mghihp.edu/node/35), [D2L: Earned awards](https://community.d2l.com/brightspace/kb/articles/22394-view-and-share-earned-awards)). Awards can be issued **manually** or **automatically via Release Conditions** (added March 2016). The **Awards Leaderboard widget** displays top-10 learners by badge count or credit total per course ([Brightspace: Awards Leaderboard widget](https://documentation.brightspace.com/EN/le/awards/admin/awards_leaderboard_widget.htm)).

### 3d. Class Progress

Per-course dashboard of up to 4 of 7 engagement metrics (content completion, login, grades, discussions, assignments, etc.) per student, with intervention affordances ([D2L: Class Progress](https://community.d2l.com/brightspace/kb/articles/3552-track-course-progress-with-the-class-progress-tool), [About Class Progress](https://community.d2l.com/brightspace/kb/articles/3314-about-class-progress)). Students see their own view too.

**Open-source.** Proprietary. The Valence API is documented but the engine is closed.

---

## 4. Moodle — Level Up XP / XP+ / Stash / Block_Game

Moodle's core has no built-in XP system, but the plugin ecosystem covers it richly.

**Level Up XP (block_xp)** by Frédéric Massart. Open-source, AGPL-style ([moodle.org plugin page](https://moodle.org/plugins/block_xp), [FMCorz/moodle-block_xp on GitHub](https://github.com/FMCorz/moodle-block_xp)).
- **Rules**: each Moodle event (view module, submit assignment, post discussion, complete activity, receive grade) maps to an XP value. Rules are editable on a Rules page with a default set; admins can override per-course.
- **Level curve**: configurable thresholds per level (linear, exponential, or custom array).
- **Multipliers**: rule-based; e.g., 2× XP for activities in a certain section.
- **Scope**: per-course tally OR site-wide tally (admin toggle).
- **Cheat Guard** ([docs.levelup.plus/cheat-guard](https://docs.levelup.plus/xp/docs/getting-started/cheat-guard)): duplicate-action suppression + max-actions-per-time-window throttle. XP+ premium adds longer time windows (weeks–months) and team leaderboards, partial anonymization, mobile-app XP visibility.
- **Companion plugin `availability_xp`** lets Moodle's standard activity-availability conditions gate on XP/level — i.e., "must be Level 5 to see this quiz." That makes XP itself a release-condition predicate.

**Stash (block_stash)** ([moodle.org/plugins/block_stash](https://moodle.org/plugins/block_stash), [FMCorz/moodle-block_stash](https://github.com/FMCorz/moodle-block_stash)). Teachers create *items* and place them in activities/resources as hidden "drops"; students collect them into an inventory. Companion `availability_stash` lets items gate further content. Newer versions add **item trading** between students ([eLearn Magazine](https://www.elearnmagazine.com/technology/moodle-gamification-now-enable-trading-of-stash-items-in-your-course-with-stash-block-moodle/)).

**block_game** ([JotaDF/moodle-block_game](https://github.com/JotaDF/moodle-block_game)) — avatar choice, score/level control, rank list, all configurable per-block. Less polished than Level Up but bundles avatars natively.

---

## 5. Schoology — Badges (and Not Much Else)

Schoology has a simple **course-scoped badge creator**: any image + name + message ([Bearded Tech-Ed Guy](https://www.beardedtechedguy.com/bring-fun-into-your-class-with-schoology-badging/), [Teched Up Teacher](https://www.techedupteacher.com/badges-rewards-schoology-and-you/)). Teachers issue manually (no automatic rules); students get a notification and the badge appears on their profile. Other students can see peers' badges. No leaderboard, no XP, no adaptive release. It's the minimum viable badge system.

---

## 6. Khan Academy — Mastery, Energy Points, Streaks

Khan Academy is structured around **per-skill mastery**, with each skill worth 100 Mastery Points distributed across discrete levels ([Khan Academy Help: Mastery levels](https://support.khanacademy.org/hc/en-us/articles/5548760867853--How-do-Khan-Academy-s-Mastery-levels-work)):

- **Attempted** (started)
- **Familiar** = 50 pts (basic exercises correct)
- **Proficient** = 80 pts (passes mastery quiz)
- **Mastered** = 100 pts (passes a mastery challenge that re-tests after time has elapsed — spaced retrieval)

Skills roll up into Unit Mastery and Course Mastery. Crucially, **scores can decay** — miss a question on a mastery challenge and you drop a level. This is closer to true mastery learning than anything Canvas/D2L ships.

**Energy Points** are a separate, monotonic "effort" counter awarded for every correct answer, video watched, etc. — pure feel-good metric, no decay ([Help Center: Mastery + Energy Points](https://support.khanacademy.org/hc/en-us/articles/9815463103245-What-happens-to-my-Mastery-and-Energy-points-when-I-start-an-activity-over)).

**Streaks** ([Streaks and Levels announcement](https://support.khanacademy.org/hc/en-us/community/posts/28945393485581-Update-Introducing-Streaks-and-Levels)) reward getting at least one skill to Proficient per week (not per day — explicitly chosen to reduce burnout).

**Parent visibility** ([Parent Dashboard](https://support.khanacademy.org/hc/en-us/articles/360039664491-What-can-I-do-from-the-Khan-Academy-Parent-Dashboard)): activity overview across all courses, drill into child's Learner Home and Khanmigo (AI tutor) chat history. Parents can link to existing learner accounts or create new ones.

---

## 7. Google Classroom — The Anti-Pattern

Google Classroom has **no native gamification, no badges, no XP, no adaptive release, no conditional content** ([Google Classroom Community: badges thread](https://support.google.com/edu/classroom/thread/6391686/can-we-gamify-our-classes-with-points-and-badges?hl=en)). Teachers improvise with Google Drawings, Slides, and Add-Ons. Treat this as the baseline of what *not* to ship if engagement matters.

---

## Synthesis: Atomic Primitives for Paper LMS

The architectural spine the user asked for. The unifying abstraction across every system above is:

> **Rule = ConditionSet (predicate over learner state) × TriggerEvent (when to evaluate) × Effect (what to do)**

Brightspace's insight is that **ConditionSet is a reusable language** — the same predicates that gate content also fire Intelligent Agents and issue Awards. Paper LMS should adopt that.

### 15–25 atomic primitives

**A. ConditionSet predicates** (the vocabulary — copy from D2L, extend with Canvas Outcomes and Khan-style mastery):

1. `SubmittedAssignment(assignment_id, [score_min, score_max])` — covers Canvas Mastery Paths bands and D2L `SubmitsToDropbox` / `ReceivesScoreOnGradeItem`.
2. `NotSubmittedAssignment(assignment_id, by_datetime?)`
3. `SubmittedQuiz(quiz_id, [score_min, score_max])` and `NotSubmittedQuiz`
4. `ViewedContent(item_id)` / `CompletedContent(item_id)` / `ViewedAllInModule(module_id)`
5. `PostedInDiscussion(topic_id, [min_posts])` / `RepliedToThread`
6. `OutcomeMastery(outcome_id, level)` — uses any of {decaying-avg, n-times, most-recent, highest, weighted-avg}. **This is the Canvas-killer.** D2L has `CompetencyAchieved`; Canvas Outcomes have the math but no trigger surface — Paper LMS unifies them.
7. `KhanStyleMasteryLevel(skill_id, ≥Familiar|≥Proficient|≥Mastered)` — same as #6 but with the spaced-retrieval decay semantics.
8. `XPThreshold(amount, scope: course|site)` / `LevelThreshold(n)` — Level Up XP availability_xp.
9. `EarnedBadge(badge_id)` — D2L `EarnsAward`.
10. `CompletedChecklist(checklist_id)` / `CompletedChecklistItem`.
11. `EnrolledIn(group_id | section_id | role)` — D2L Classlist family.
12. `DaysSinceEnrollment(n)` and `DaysSinceLastLogin(n)` — D2L `DaysEnrolledInCurrentOrgUnit` + Intelligent Agent login criterion.
13. `DateWindow(start, end)` — absolute date gate (release date / unlock date).
14. `RelativeDateFromEnrollment(offset)` — rolling-enrollment release.
15. `ReceivedRubricRating(rubric_id, criterion_id, ≥level)` — drives mastery from rubric hits, not just totals.

**B. Composition** (one primitive, hugely powerful):

16. `ConditionSet(op: AND|OR|N_OF_M, children: [predicate | ConditionSet])` — recursive boolean tree with **N-of-M** as a first-class op (e.g., "completed any 3 of 5 practice problems"). Brightspace only does AND/OR at the rule level; Paper LMS should ship N-of-M from day one.

**C. TriggerEvent** (when the rule engine re-evaluates):

17. `OnLearnerAction(action_type)` — submission, view, post, login, etc. (real-time evaluation; this is Level Up XP's model).
18. `OnGradePosted(assignment_id)` — what Canvas Mastery Paths fires on.
19. `OnSchedule(cron)` — Intelligent Agents' periodic sweep.
20. `OnManualTrigger(actor)` — teacher button.

**D. Effect** (the action taken when ConditionSet evaluates true under a TriggerEvent):

21. `ReleaseContent(item_id | module_id)` — strict superset of Mastery Paths' assignment-override mechanism. Implementation tip: use Canvas's pattern — synthesize an assignment/section override rather than maintaining a separate visibility table.
22. `AwardXP(points, multiplier?)` with cheat-guard parameters (max-per-window, dedupe-key).
23. `AwardBadge(badge_id)` / `AwardCertificate(template_id)` — Open Badges 2.0 + PDF.
24. `AdvanceRankOrLevel` (derived from XP threshold; emit as separate effect for stash/avatar/title changes).
25. `Notify(recipients, template, channel: email|in_app|push)` — Intelligent Agents' core effect.
26. `BranchPath(next_item | path_label)` — explicit branching that Mastery Paths only does implicitly via overrides. Lets a teacher author a DAG, not just a 3-band fan-out.
27. `EnrollInGroup(group_id)` — auto-cohort by performance (D2L can do this via Agents → enrollment changes). Powerful for "remediation group" automation.
28. `DepositStashItem(item_id)` — Moodle Stash parity, optional.

### Where this beats every system surveyed

- **Beats Canvas Mastery Paths**: arbitrary boolean trees + N-of-M (not just 3 score bands), branching as an explicit effect, mastery-percentage as a trigger source (Canvas can't do that today), date/enrollment gates.
- **Beats Brightspace**: shared predicate language *plus* native XP/level primitives (D2L has Awards but no XP curve), plus N-of-M, plus the Khan-style decay semantics for mastery.
- **Beats Moodle Level Up**: Level Up bolts onto the existing availability system; Paper LMS bakes XP/level into the same predicate vocabulary as content release and badge issuance — one engine, not three plugins.
- **Beats Schoology**: anything beats Schoology.
- **Beats Khan Academy**: keep their mastery decay + streak semantics, but expose them as predicates teachers can compose, not a fixed K-12 pipeline.
- **Beats Google Classroom**: trivially.

### Implementation note for Paper LMS

The data model on disk should be something like:

```
rules (id, course_id, name, enabled, trigger_event jsonb, condition_set jsonb, effects jsonb[])
rule_evaluations (rule_id, user_id, evaluated_at, result, fired)
```

`condition_set` is a recursive JSONB tree mirroring primitive 16. Re-evaluation is event-driven (subscribe to submission/grade/view events) plus a nightly cron sweep for date/enrollment predicates. This collapses Canvas's `assignment_overrides` table, D2L's release-condition table, and a hypothetical XP-rule table into one unified rules engine — which is the architecturally elegant outcome the user said they want.

---

## Sources

- [Instructure Community — How do I use Mastery Paths in course modules?](https://community.canvaslms.com/t5/Instructor-Guide/How-do-I-use-Mastery-Paths-in-course-modules/ta-p/906)
- [Instructure — A Quick Guide to Creating Mastery Paths](https://www.instructure.com/resources/blog/quick-guide-creating-mastery-paths)
- [Instructure — How do I add conditional content to a Mastery Path source item?](https://community.instructure.com/en/kb/articles/660916-how-do-i-add-conditional-content-to-a-mastery-path-source-item)
- [Canvas LMS REST API — Assignments](https://developerdocs.instructure.com/services/canvas/resources/assignments)
- [instructure/canvas-lms on GitHub](https://github.com/instructure/canvas-lms)
- [Instructure — Outcomes: New and Updated Proficiency Calculations](https://community.canvaslms.com/t5/The-Product-Blog/Canvas-Outcomes-New-and-Updated-Proficiency-Calculations-Coming/ba-p/579866)
- [D2L Community — About release conditions](https://community.d2l.com/brightspace/kb/articles/3432-about-release-conditions)
- [Valence Developer Platform — Release Conditions reference](https://docs.valence.desire2learn.com/res/releaseconditions.html)
- [Brightspace Knowledge Base — Release Conditions (NICC)](https://sites.google.com/nicc.edu/brightspaceknowledgebase/instructor-tutorials/instructor-quick-reference-guide/release-conditions)
- [D2L Community — About Intelligent Agents](https://community.d2l.com/brightspace/kb/articles/3499-about-intelligent-agents)
- [D2L Community — Set up Intelligent Agents](https://community.d2l.com/brightspace/kb/articles/3496-set-up-intelligent-agents)
- [Brightspace Help — Awards Tool](https://d2lhelp.mghihp.edu/node/35)
- [Brightspace — Awards Leaderboard widget](https://documentation.brightspace.com/EN/le/awards/admin/awards_leaderboard_widget.htm)
- [D2L Community — About Class Progress](https://community.d2l.com/brightspace/kb/articles/3314-about-class-progress)
- [Moodle Plugins — Level Up XP (block_xp)](https://moodle.org/plugins/block_xp)
- [FMCorz/moodle-block_xp on GitHub](https://github.com/FMCorz/moodle-block_xp)
- [Level Up XP docs — Cheat Guard](https://docs.levelup.plus/xp/docs/getting-started/cheat-guard)
- [Moodle Plugins — Level Up XP Availability](https://moodle.org/plugins/availability_xp)
- [Moodle Plugins — Stash (block_stash)](https://moodle.org/plugins/block_stash)
- [FMCorz/moodle-block_stash on GitHub](https://github.com/FMCorz/moodle-block_stash)
- [JotaDF/moodle-block_game on GitHub](https://github.com/JotaDF/moodle-block_game)
- [Bearded Tech-Ed Guy — Schoology Badging](https://www.beardedtechedguy.com/bring-fun-into-your-class-with-schoology-badging/)
- [Teched Up Teacher — Badges, Rewards, Schoology, and You](https://www.techedupteacher.com/badges-rewards-schoology-and-you/)
- [Khan Academy Help — Mastery levels](https://support.khanacademy.org/hc/en-us/articles/5548760867853--How-do-Khan-Academy-s-Mastery-levels-work)
- [Khan Academy Help — Course and Unit Mastery](https://support.khanacademy.org/hc/en-us/articles/115002552631-What-are-Course-and-Unit-Mastery)
- [Khan Academy — Streaks and Levels announcement](https://support.khanacademy.org/hc/en-us/community/posts/28945393485581-Update-Introducing-Streaks-and-Levels)
- [Khan Academy Help — Parent Dashboard](https://support.khanacademy.org/hc/en-us/articles/360039664491-What-can-I-do-from-the-Khan-Academy-Parent-Dashboard)
- [Google Classroom Community — Can we gamify with points and badges?](https://support.google.com/edu/classroom/thread/6391686/can-we-gamify-our-classes-with-points-and-badges?hl=en)
