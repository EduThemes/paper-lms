# Behavioral Design Frameworks for Paper LMS Gamification

*Claude research agent, 2026-05-12. Focus: Octalysis, Duolingo old/new, SDT, Flow, streaks, variable rewards, leaderboard research, dark patterns.*

A research synthesis to ground Paper LMS triggers in motivation science. Target audience: design and engineering leads making ship/no-ship calls on individual mechanics for K-5 through Higher Ed.

---

## 1. The Octalysis Framework (Yu-kai Chou)

Octalysis maps human motivation to **8 Core Drives**, arranged as an octagon. Top three (CD1, CD2, CD3) are **White Hat** (make users feel powerful, in control, fulfilled — drive long-term engagement). Bottom three (CD6, CD7, CD8) are **Black Hat** (urgency, obsession, anxiety — drive short-term action but burn out users). CD4 and CD5 are neutral. The left side (CD1, 2, 4, 6) is **left-brain / extrinsic**; right side (CD3, 5, 7, 8) is **right-brain / intrinsic** ([yukaichou.com](https://yukaichou.com/gamification-examples/octalysis-gamification-framework/), [Wikipedia](https://en.wikipedia.org/wiki/Octalysis)).

### CD1 — Epic Meaning & Calling [White Hat / Extrinsic]
*"You were chosen. This matters beyond you."*

**Game mechanics:** Narrative framing, beginner's luck, "chosen one" storylines, contribution-to-a-bigger-cause (e.g. Wikipedia editors), heritage symbols, elite-collective belonging.

**Paper LMS triggers:**
- Class mission statement on dashboard ("This class is learning to write so we can publish a real book.")
- Contribution counters: "Your class has answered 14,302 problems together this year."
- Teacher-authored welcome video / course identity on first login.
- Public good framing for service-learning units.
- Beginner badge ("Pioneer of Mr. K's Algebra 1 Cohort 2026") given automatically on first lesson.

**Ethics flag:** Safe across all ages. The K-5 version should be teacher-curated, not algorithm-generated, to avoid hollow "Hero!" framing.

### CD2 — Development & Accomplishment [White Hat / Extrinsic]
*"I am getting better. I am leveling up."*

**Game mechanics:** Points, badges, leaderboards (PBL), progress bars, level ups, completion checkmarks, mastery rubrics, milestone celebrations.

**Paper LMS triggers:**
- Module progress bars + per-objective mastery indicators.
- Earned badges tied to learning outcomes, not just minutes spent.
- Score-vs-self trend lines (improvement over time, not class rank).
- Section/Unit checkpoint celebrations (confetti animation, "you finished Unit 3").
- Mastery rubric "growth" visualization across a marking period.
- XP that maps to *what you learned*, not *how long you stared at the page*.

**Ethics flag:** Generally safe. Risk: making points-per-second the dominant signal turns into a grind — keep CD2 tied to mastery checkpoints, not raw time.

### CD3 — Empowerment of Creativity & Feedback [White Hat / Intrinsic]
*"I get to make and decide things, and I see the result immediately."*

**Game mechanics:** Sandbox / construction kits, customization, real-time feedback, "what-if" simulators, branching choices, UGC tools, immediate reaction to action.

**Paper LMS triggers:**
- Branching assignments where the student picks the topic / artifact format (essay vs. video vs. infographic).
- Inline auto-feedback on writing (grammar, structure) while drafting.
- Math word-problem generator that re-rolls until the student is interested (your old randomized engine — flow + CD3 combined).
- Avatar/portfolio customization tied to learning identity, not paywalled cosmetics.
- Open-ended rubrics with student-chosen exemplars.
- Discussion replies show typing indicator + immediate teacher emoji reactions.

**Ethics flag:** Strongest white-hat lever for long-term intrinsic motivation. **Prioritize over CD2.**

### CD4 — Ownership & Possession [Neutral / Extrinsic]
*"This is mine. I built it. I want to protect and grow it."*

**Game mechanics:** Virtual currency, avatars, collections, inventory, upgrade trees, profile customization, "my learning garden."

**Paper LMS triggers:**
- Personal portfolio page where each completed assignment becomes a visible artifact.
- Collectible knowledge cards (one per concept mastered) — student "owns" their concept deck.
- Customizable dashboard themes (light/dark/paper modes — fits the project aesthetic).
- "My learning garden" / "knowledge tree" visual where each branch is a mastered standard.
- Student-owned reflection journal that persists across years.

**Ethics flag:** Safe if the "possessions" are *learning artifacts* rather than purchased cosmetics. Avoid: anything spendable from a wallet that requires real-world money for kids.

### CD5 — Social Influence & Relatedness [Neutral / Intrinsic]
*"My peers / teacher / family see me, and I see them."*

**Game mechanics:** Mentorship, peer review, social leaderboards, gifting, friend lists, group challenges, "your friend just achieved X" notifications, nostalgia.

**Paper LMS triggers:**
- Small-cohort leaderboards (max ~30, weekly reset — Duolingo style).
- Peer review with structured rubric, mutual feedback.
- Class-vs-class team challenges (whole-cohort effort, not individual).
- Parent/guardian "look what I did" share button (parent-only audience).
- Teacher kudos: a private "well done" emoji from teacher arrives as a notification.
- Friends Quests-style pair challenges with consent.

**Ethics flag:** Moderate. Social-comparison rankings are demotivating for the bottom 80% of any cohort (see leaderboard research below). Make rankings **relative** and **time-bounded**.

### CD6 — Scarcity & Impatience [Black Hat / Extrinsic]
*"I can't have it now. I want it more because of that."*

**Game mechanics:** Appointment dynamics, limited-time offers, exclusive tiers, VIP gates, locked content with countdown, "back tomorrow."

**Paper LMS triggers (use sparingly):**
- Daily lesson drops (don't release all of Unit 4 at once — pace it).
- Limited-time bonus challenges (weekend brain-teaser that disappears Monday).
- Teacher-set "office hours" appointment dynamics.

**Ethics flag:** **High risk for K-12.** Manufactured scarcity (fake countdowns, FOMO timers) is a documented dark pattern that exploits immature executive function ([fairplayforkids.org](https://fairplayforkids.org/wp-content/uploads/2021/05/darkpatterns.pdf)). Default OFF for K-5; default OFF for K-8 unless teacher opts in.

### CD7 — Unpredictability & Curiosity [Black Hat / Intrinsic]
*"I wonder what happens next."*

**Game mechanics:** Variable reward schedules, loot boxes, mystery boxes, lottery, surprise encounters, easter eggs, random encounters.

**Paper LMS triggers (ethical version):**
- Surprise "mystery bonus XP" (5–25 XP) on random correct answers — *informational*, not paywalled.
- Easter-egg achievements ("you found the hidden study tip").
- Randomized question order to keep practice fresh.
- Surprise teacher voice notes occasionally embedded in lessons.
- Randomized math word-problem generation (also serves flow).

**Ethics flag:** **Loot boxes for purchase are banned for minors in Belgium, the Netherlands, and increasingly restricted elsewhere.** Never gate rewards behind real or in-app currency for K-12. Variable rewards must be earned through learning behavior, not purchased.

### CD8 — Loss & Avoidance [Black Hat / Extrinsic]
*"I can't lose what I've built."*

**Game mechanics:** Streaks-at-risk, sunk-cost progress decay, league demotion, "you'll lose your spot," fading opportunities, hearts/lives running out, status removal.

**Paper LMS triggers (use cautiously):**
- Streaks with **auto-freeze on weekends and holidays** for K-12.
- Spaced-repetition "memory decay" indicators that frame as *opportunity to refresh*, not *threat of loss*.
- Mastery decay warnings ("review this skill before Friday's quiz").

**Ethics flag:** **Highest risk drive for K-12.** Documented sources of streak anxiety, guilt-driven engagement, identity attachment ([drracheltaylor.substack.com](https://drracheltaylor.substack.com/p/why-my-daughter-quit-duolingo-the), [duolingoguides.com](https://duolingoguides.com/why-duolingo-is-scary-the-psychology-behind-that-green-owl/)). Reframe as gain, not loss. Default OFF for K-5.

### Why an LMS for kids must lean white-hat

Black Hat mechanics work — they drive short-term DAU spikes — but research on children's apps shows they exploit immature executive function, induce anxiety, and create guilt-based engagement ([Fair Play for Kids FTC filing](https://fairplayforkids.org/wp-content/uploads/2021/05/darkpatterns.pdf), [Dark Patterns in Early Childhood Mobile Gaming](https://papers.academic-conferences.org/index.php/ecgbl/article/download/1656/1700/6569)). Sustained learning requires intrinsic motivation, which black-hat undermines via the overjustification effect (see §4). **Default white-hat. Make black-hat opt-in by school admin, never default-on for under-13.**

---

## 2. Duolingo — Old System (Pre-2020)

The classic skill-tree era ([duolingo.fandom.com/wiki/Language_tree](https://duolingo.fandom.com/wiki/Language_tree), [theflyy.com](https://www.theflyy.com/blog/gamification-in-duolingo)):

- **Hearts (lives):** 3–5 hearts per session. Wrong answer = lose a heart. Out of hearts = locked out or pay/practice.
- **Lingots:** Virtual currency earned for level-ups and streak milestones. Spent on streak freezes, bonus skills, double-or-nothing wagers, costumes for Duo.
- **Skill tree:** Branching dependency graph. Skill A must be unlocked before Skill B. Visual tree on home screen.
- **Strength bars / decay:** Each skill had a strength bar (1–5) that decayed over time. Required "Practice" to restore — a loss-aversion mechanic dressed as spaced repetition.
- **Crowns (added 2018):** Each skill could be leveled 0–5 crowns (eventually 6). Replaced linear "completion" with vertical mastery.
- **Streaks:** Consecutive days hitting daily XP goal. Streak Freeze (purchasable for ~10 lingots) protected one missed day.
- **Clubs:** Small social cohorts (~15 people) with shared XP leaderboards. Removed in 2020.
- **Double-or-nothing:** Wager 5 lingots, get 10 if you keep a 7-day streak. Pure gambling mechanic.
- **Daily XP goal:** User-configurable (Casual=10, Regular=20, Serious=30, Insane=50).
- **Achievement badges:** Trophy collection on profile.
- **Immersion (later removed):** Crowdsourced translation contributions — Epic Meaning play.

---

## 3. Duolingo — Current System (2024–2026)

Major changes documented at [blog.duolingo.com](https://blog.duolingo.com/new-duolingo-home-screen-design/), [duoplanet.com](https://duoplanet.com/duolingo-new-learning-path-review/), and [Deconstructor of Fun](https://www.deconstructoroffun.com/blog/2025/4/14/duolingo-how-the-15b-app-uses-gaming-principles-to-supercharge-dau-growth):

- **The Path (Nov 2022):** Linear sequence replacing the branching tree. One step at a time. Reduces decision paralysis, increases completion rates. Crowns folded into path levels (1 crown = 1 step). Survived A/B test despite vocal forum backlash.
- **XP:** Still the universal currency for league climb. Score (new, 2024) only goes up via path progression, no farming.
- **Leagues:** 10 tiers — Bronze, Silver, Gold, Sapphire, Ruby, Emerald, Amethyst, Pearl, Obsidian, Diamond. **30 users per league**, matched by timezone and study habits. Weekly reset Sunday. **Top 7 promote, bottom 5 demote** (numbers vary by tier). Diamond Tournament for top 10 of Diamond ([blog.duolingo.com/duolingo-leagues-leaderboards](https://blog.duolingo.com/duolingo-leagues-leaderboards/)).
- **Super Duolingo:** Paywall ($7/mo). Unlimited hearts, no ads, monthly streak repair, Legendary access.
- **Daily Quests:** 3 small tasks reset daily. Completing all 3 = chest reward (gems + XP boost).
- **Friends Quests:** Weekly random pairing with a friend, shared goal, 5 days, joint reward (100 gems + 30 min 2x XP) ([blog.duolingo.com/friends-quests](https://blog.duolingo.com/friends-quests/)).
- **Family Plan:** Up to 6 accounts, $10/mo — Social-Influence-driven retention.
- **Streak Society:** Automatic membership at 7+ day streak. Cosmetic streak flame upgrade, exclusive rewards.
- **Streak Freeze / Repair:** Up to 2 freezes equipped. Repair (paid) restores broken streaks.
- **Practice Hub:** Free-form review (Stories, Listening, Speak, Match Madness) — Super-only for most modes.
- **Energy (rolling out 2025):** Replaces Hearts with a 25-unit battery that depletes per *question* (right or wrong) rather than per mistake. **Highly controversial** — users describe it as a cash grab forcing Super conversion after ~3 free lessons ([toptechguides.com](https://toptechguides.com/duolingo-energy-update-backlash/), [androidauthority.com](https://www.androidauthority.com/quitting-duolingo-energy-system-3599842/)). Still in A/B test as of mid-2025.
- **Combos / Match Madness:** Streak-within-a-session mechanic — consecutive correct answers earn bonus XP. Pure flow / CD7 unpredictability.
- **Legendary Level:** Per-unit (previously per-skill). Purple crown. Hardest difficulty, no hints, limited hearts. Confirms mastery.
- **Section / Unit Checkpoints:** Cinematic story beats with characters (Lily, Bea, Junior). Narrative spacing.
- **Owl notifications:** Aggressively guilt-framed ("You made Duo sad 😢"). Effective in A/B tests, ethically questionable.
- **Hub characters / story:** Soap-opera narrative threading the path. Drives CD1 (epic meaning) for the brand itself.

**What survived A/B tests:** Path, Leagues, Streak Society, Friends Quests, guilt-notifications.
**What didn't:** Clubs, Immersion, Comments on lessons, Discussion forums (mostly killed for retention reasons).
**Controversial but kept:** Streak guilt, owl memes, Energy (still in test as of this Claude agent's run — see parallel AI's Appendix A.4 for confirmed May 2026 status).

---

## 4. Self-Determination Theory (Deci & Ryan)

SDT proposes three innate psychological needs whose satisfaction predicts both well-being and durable motivation ([selfdeterminationtheory.org](https://selfdeterminationtheory.org/theory/), [Wikipedia](https://en.wikipedia.org/wiki/Self-determination_theory)):

| Need | Definition | LMS Triggers |
|---|---|---|
| **Autonomy** | Feeling that you choose your actions | Student picks assignment topic, format, due date within a window; optional "stretch" problems; choice of avatar; opt-in challenges; **never auto-enroll into leaderboards** |
| **Competence** | Feeling effective at meaningful tasks | Adaptive difficulty; mastery-based progression; immediate informational feedback; visible skill tree; "you got 4/5 — here's what you missed" framing |
| **Relatedness** | Feeling connected to others who matter | Teacher voice notes; small-cohort discussions; peer review; family share button; classroom Slack-style channels; teacher emoji reactions |

### The Overjustification Effect (Deci 1971)

Deci's classic SOMA-cube experiment: paid puzzle-solvers showed less subsequent interest than unpaid ones. Extrinsic rewards can **crowd out** intrinsic motivation for already-interesting tasks ([Wikipedia](https://en.wikipedia.org/wiki/Overjustification_effect), [Deci/Koestner/Ryan meta-analysis 1999](https://home.ubalt.edu/tmitch/642/articles%20syllabus/Deci%20Koestner%20Ryan%20meta%20IM%20psy%20bull%2099.pdf)). The undermining effect is **strongest in school-aged children**.

**Design patterns that avoid it:**
1. **Informational feedback > controlling feedback.** "You demonstrated mastery of fractions" (informational) beats "Good job! Here's your sticker" (controlling) ([structural-learning.com](https://www.structural-learning.com/post/overjustification-effect)).
2. **Unexpected rewards > expected rewards.** Surprise badges don't crowd out intrinsic motivation; promised "if-then" rewards do.
3. **Reward effort/strategy, not just outcomes.** Carol Dweck's growth-mindset overlap.
4. **Make rewards tied to mastery, not time.** "You unlocked this because you can do it" beats "You unlocked this because you spent 30 minutes."
5. **Withdraw rewards gracefully.** Once a student is engaged, fade the badge spam. Avoid lifelong dependency on points.

---

## 5. Flow Theory (Csikszentmihalyi)

Flow = total absorption when **perceived challenge ≈ perceived skill, slightly above current ability** ([Wikipedia](https://en.wikipedia.org/wiki/Flow_(psychology)), [growthengineering.co.uk](https://www.growthengineering.co.uk/flow-theory/)). Too hard → anxiety. Too easy → boredom. Flow channel must ascend as skill grows.

**Adaptive difficulty applied to LMS:**
- Your old randomized math word-problem engine is already a flow engine. Add a per-student difficulty estimator (Elo-style) so each generated problem targets ~80% expected success.
- IRT-based item selection from a question bank (used by NWEA MAP, Khan Academy).
- Difficulty bands per learning objective: easy / on-grade / stretch. Default on-grade, auto-bump after 3 in a row correct.

**Signals an LMS can use to classify state:**
| State | Time-on-task | Error rate | Behavior |
|---|---|---|---|
| **Flow** | Long, steady | 15–25% (productive struggle) | No idle, low hint usage |
| **Boredom** | Short, fast | <5% | Speed-clicking, skipping reading |
| **Frustration** | Long, erratic | >50% | Repeated wrong, multiple hint pulls, abandonment |
| **Anxiety** | Variable | Increasing over session | Quit/restart, refresh, idle then frantic |

**Triggers when frustration detected:** Auto-offer easier alternate problem; surface hint scaffold; suggest a break; ping teacher dashboard.
**Triggers when boredom detected:** Bump difficulty; offer stretch challenge; switch modality.

---

## 6. Streak Design Research

Duolingo's streak is simultaneously their **#1 retention driver and most-complained-about feature** ([Deconstructor of Fun analysis](https://www.deconstructoroffun.com/blog/2025/4/14/duolingo-how-the-15b-app-uses-gaming-principles-to-supercharge-dau-growth)). What works vs. what hurts:

**Engaging:**
- Visible counter creates identity ("I am a 90-day streaker").
- Streak Freeze removes catastrophic loss aversion.
- Streak Society (social proof at 7+ days) leverages CD5 not CD8.
- Friend Streaks make breaking it feel like letting a peer down — drives co-accountability.

**Anxiety-inducing:**
- Hard reset to zero after one miss creates disproportionate panic.
- Push notifications timed to evening with guilt framing ("You made Duo sad") ([Why Duolingo Is Scary](https://duolingoguides.com/why-duolingo-is-scary-the-psychology-behind-that-green-owl/)).
- Identity attachment turns missed days into self-criticism, especially for children ([Dr. Rachel Taylor's substack](https://drracheltaylor.substack.com/p/why-my-daughter-quit-duolingo-the)).
- No weekend amnesty for kids whose schedules aren't theirs to control.

**Paper LMS streak design rules:**
1. Auto-freeze weekends and holidays for K-12.
2. Auto-freeze when device is offline (no internet ≠ no learning).
3. Allow up to 2 "free" freezes/month, no purchase.
4. Reset notifications must be neutral / encouraging, never guilt.
5. K-5: streaks are **opt-in by teacher**, never visible by default.
6. Frame as "learning days this month" instead of "DON'T BREAK THE STREAK."

---

## 7. Variable Reward Schedules

B.F. Skinner showed intermittent reinforcement (variable ratio) produces the strongest, most persistent behavior — pigeons peck the lever harder when reward is unpredictable ([nirandfar.com](https://www.nirandfar.com/want-to-hook-your-users-drive-them-crazy/)). Nir Eyal's *Hooked* model embeds this as the third phase: **trigger → action → variable reward → investment** ([Mindtools](https://www.mindtools.com/aapqtdb/the-hook-model-of-behavioral-design/)).

**Three types of variable reward (Eyal):**
- **Tribe:** Unpredictable social validation (likes, comments).
- **Hunt:** Unpredictable resources (loot, content discovery).
- **Self:** Unpredictable mastery (level-up surprises).

**Why loot boxes are regulated:**
- Belgium (2018), Netherlands, and parts of the EU classify paid loot boxes as gambling for minors.
- UK and Australia have ongoing inquiries.
- Apple/Google require disclosure of odds.
- Documented dark pattern when targeted at children ([Hooked on Loot Boxes / Medium](https://medium.com/behavior-design/hooked-on-loot-boxes-how-design-gets-us-addicted-79c45faebc05)).

**Ethical use of variable rewards in an LMS:**
- Variable amount of XP per correct answer (5–25 XP, hidden distribution).
- Surprise badges on milestones the student didn't know existed.
- Random easter-egg encouragement messages from teacher.
- Mystery-box openings tied to *mastery completion*, never *purchase*.
- Never let a real-money or even in-app-currency transaction gate a random reward for under-18.

---

## 8. Leaderboard Research

The honest finding from peer-reviewed work ([Li 2024, Journal of Computer Assisted Learning](https://onlinelibrary.wiley.com/doi/10.1111/jcal.13077?af=R), [ScienceDirect longitudinal study](https://www.sciencedirect.com/science/article/abs/pii/S1041608024001651)):

**What works:**
- Small cohort (20–30 users — Duolingo settled on 30 for a reason).
- Relative leaderboards showing only nearby ranks (your position ±5), not global.
- Time-bounded resets (weekly), giving every user a fresh chance.
- Effort-based metrics (XP earned, problems attempted) rather than ability-based (raw score).
- Tiered promotion/demotion so similar-skill peers compete.

**What backfires:**
- Global, all-time leaderboards — bottom 80% disengage permanently.
- Public rank displayed alongside name in front of class.
- Ability-based ranking that locks low-skill students out structurally.
- Forced participation without opt-out.

**Why the user wants relative leaderboards:** A relative leaderboard lets a struggling student see "I can pass these 3 people this week" rather than "I'm 217th out of 240 forever." This converts demoralizing upward social comparison into achievable proximate goals — directly addressing the competence-frustration finding ([Emerald Insight study](https://www.emerald.com/insight/content/doi/10.1108/intr-12-2021-0897/full/html)).

---

## 9. Anti-Patterns to Avoid

- **Pay-to-win:** Any mechanic where money buys learning outcomes (extra hearts, easier quizzes, mastery shortcuts).
- **Manipulative notifications:** Guilt framing, fake urgency, "Duo is sad" passive-aggression. K-12 push notifications should be neutral and frequency-capped.
- **Loss-framed streaks for young kids:** "You'll LOSE your streak!" → reframe as "Keep your learning days going."
- **Public shaming:** Bottom-of-class leaderboards, public failure announcements, "Wall of Shame" — never.
- **Engagement-for-engagement's-sake:** If a mechanic boosts DAU but not learning outcomes, it's a dark pattern. Tie every trigger to a measurable learning metric.
- **Manufactured scarcity / fake countdowns:** Documented dark pattern targeting cognitively immature children ([Fair Play for Kids FTC filing](https://fairplayforkids.org/wp-content/uploads/2021/05/darkpatterns.pdf)).
- **Sunk-cost decay:** Erasing weeks of mastery because of a missed week.
- **Auto-renew traps** on Super-style paywalls — illegal in some jurisdictions for minors.
- **Identity dependency:** "Diamond League member" branded so heavily the student's self-worth depends on a tier.

---

## Trigger Design Principles (the ship/no-ship checklist)

1. **Default white-hat.** CDs 1–3 are always-on. CDs 6–8 require explicit opt-in at the school admin level.
2. **Tie every reward to learning, not time.** XP is earned through demonstrated mastery, never through minutes spent.
3. **Prefer informational feedback over controlling rewards.** "You mastered fractions" > "Good job! +50 points!"
4. **Use surprise, not promise.** Unexpected rewards don't crowd out intrinsic motivation; expected if-then rewards do.
5. **Reset leaderboards weekly. Cap cohorts at ~30. Show relative ranks ±5, never global all-time.**
6. **Cap streak anxiety with auto-freeze.** Weekends, holidays, illness flags, offline status — all auto-freeze for K-12.
7. **Make black-hat mechanics opt-in by school, never default-on under 13.** Scarcity, loss, FOMO must be teacher-curated.
8. **Frame as gain, not loss.** "Earn back your streak" not "Don't lose your streak."
9. **Adaptive difficulty by default.** Aim for ~80% success rate per student per task. Detect frustration, intervene early.
10. **Notification budget per learner per day.** Hard cap (~2 for K-5, ~4 for 9-12). Neutral tone. No guilt.
11. **Every cosmetic / collectible is earned, never purchased.** No real-money or in-app-currency loot for under-18.
12. **Choice is a feature, not a leak.** Offer at least one meaningful choice (topic, format, due date) on every multi-day assignment.
13. **Public is opt-in. Private is default.** Only the student sees their dashboard unless they choose to share with class.
14. **Show effort metrics alongside outcome metrics.** "You attempted 30 problems this week" matters as much as "You scored 85%."
15. **Sunset mechanics that stop working.** A/B test retention by *learning outcomes*, not by DAU. Kill triggers that boost DAU without moving the academic needle.

---

## K-12 vs Higher Ed Toggle Matrix

Legend: ✅ default-on · ⚙️ default-off, opt-in · ❌ never ship

| Mechanic | K-5 | 6-8 | 9-12 | Higher Ed | Corporate |
|---|:---:|:---:|:---:|:---:|:---:|
| Progress bars / unit checkpoints | ✅ | ✅ | ✅ | ✅ | ✅ |
| Mastery badges (informational) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Adaptive difficulty | ✅ | ✅ | ✅ | ✅ | ✅ |
| Customizable avatar / dashboard | ✅ | ✅ | ✅ | ⚙️ | ⚙️ |
| Knowledge garden / portfolio | ✅ | ✅ | ✅ | ✅ | ⚙️ |
| Streaks (with auto-freeze) | ⚙️ | ⚙️ | ✅ | ✅ | ⚙️ |
| Streaks (no freeze) | ❌ | ❌ | ⚙️ | ⚙️ | ⚙️ |
| Daily quests | ⚙️ | ✅ | ✅ | ✅ | ✅ |
| Small-cohort weekly leaderboards (relative) | ⚙️ | ⚙️ | ✅ | ⚙️ | ✅ |
| Global / all-time leaderboards | ❌ | ❌ | ❌ | ⚙️ | ⚙️ |
| Friends Quests / pair challenges | ⚙️ | ✅ | ✅ | ✅ | ✅ |
| Class-vs-class team challenges | ✅ | ✅ | ✅ | ⚙️ | ⚙️ |
| Variable bonus XP (surprise) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Mystery boxes (earned only) | ⚙️ | ✅ | ✅ | ⚙️ | ⚙️ |
| Hearts / energy / lives | ❌ | ⚙️ | ⚙️ | ⚙️ | ❌ |
| Hearts pay-to-refill | ❌ | ❌ | ❌ | ❌ | ❌ |
| Limited-time challenges (24–72h) | ❌ | ⚙️ | ⚙️ | ✅ | ✅ |
| Countdown timers (manufactured) | ❌ | ❌ | ❌ | ❌ | ❌ |
| Loss-framed notifications | ❌ | ❌ | ❌ | ⚙️ | ⚙️ |
| Push notifications (capped, neutral) | ⚙️ (parent) | ⚙️ | ✅ | ✅ | ✅ |
| Peer review (structured) | ⚙️ | ✅ | ✅ | ✅ | ✅ |
| Parent/guardian share button | ✅ | ✅ | ⚙️ | ❌ | ❌ |
| Public rank display | ❌ | ❌ | ⚙️ | ⚙️ | ⚙️ |
| Premium paywall / pay-to-win | ❌ | ❌ | ❌ | ⚙️ | ⚙️ |
| Real-money cosmetics | ❌ | ❌ | ❌ | ❌ | ❌ |
| Spaced-repetition decay (informational) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Spaced-repetition decay (loss-framed) | ❌ | ❌ | ⚙️ | ✅ | ✅ |
| League demotion (Duolingo-style) | ❌ | ❌ | ⚙️ | ⚙️ | ⚙️ |
| Narrative / epic-meaning framing | ✅ (teacher) | ✅ | ✅ | ⚙️ | ⚙️ |
| Choice of assignment format | ✅ | ✅ | ✅ | ✅ | ✅ |

---

## Sources

- [The Octalysis Framework — Yu-kai Chou](https://yukaichou.com/gamification-examples/octalysis-gamification-framework/)
- [White Hat vs Black Hat Gamification — Yu-kai Chou](https://yukaichou.com/gamification-study/white-hat-black-hat-gamification-octalysis-framework/)
- [Octalysis — Wikipedia](https://en.wikipedia.org/wiki/Octalysis)
- [Core Drive 8: Loss & Avoidance — Yu-kai Chou](https://yukaichou.com/gamification-study/8-loss-and-avoidance/)
- [The Octalysis Framework — Octalysis Group](https://octalysisgroup.com/framework/)
- [Gamification in Duolingo — The Flyy](https://www.theflyy.com/blog/gamification-in-duolingo)
- [Duolingo gamification explained — StriveCloud](https://www.strivecloud.io/blog/gamification-examples-boost-user-retention-duolingo)
- [How Duolingo Leaderboards and Leagues Work — Duolingo Blog](https://blog.duolingo.com/duolingo-leagues-leaderboards/)
- [Introducing the new Duolingo learning path — Duolingo Blog](https://blog.duolingo.com/new-duolingo-home-screen-design/)
- [Introducing Friends Quests — Duolingo Blog](https://blog.duolingo.com/friends-quests/)
- [Duolingo Levels — Duoplanet](https://duoplanet.com/duolingo-levels/)
- [Duolingo Legendary Level Challenges — Duoplanet](https://duoplanet.com/duolingo-legendary-levels-get-to-know-the-purple-crowns/)
- [Duolingo Energy System Complete Guide — Duoplanet](https://duoplanet.com/duolingo-energy-system/)
- [Duolingo Breaks Hearts for Energy — Class Central](https://www.classcentral.com/report/duolingo-breaks-hearts-for-energy/)
- [Duolingo Users Revolt Over Energy Update — Top Tech Guides](https://toptechguides.com/duolingo-energy-update-backlash/)
- [How Duolingo's New Energy System Is Failing Its Users — Sam Liberty / Medium](https://medium.com/design-bootcamp/how-duolingos-new-energy-system-is-failing-its-users-16738c83117b)
- [Duolingo: How the $15B App uses Gaming Principles — Deconstructor of Fun](https://www.deconstructoroffun.com/blog/2025/4/14/duolingo-how-the-15b-app-uses-gaming-principles-to-supercharge-dau-growth)
- [Why Duolingo Is Scary: The Psychology Behind That Green Owl — Duolingo Guides](https://duolingoguides.com/why-duolingo-is-scary-the-psychology-behind-that-green-owl/)
- [Why My Daughter Quit Duolingo: The Neuroscience of Streak Addiction — Dr. Rachel Taylor](https://drracheltaylor.substack.com/p/why-my-daughter-quit-duolingo-the)
- [Self-Determination Theory — Wikipedia](https://en.wikipedia.org/wiki/Self-determination_theory)
- [Self-Determination Theory — Official Site](https://selfdeterminationtheory.org/theory/)
- [Ryan and Deci 2020 SDT paper](https://stial.ie/resources/Ryan%20and%20Deci%202020%20self%20determination%20theory.pdf)
- [Overjustification Effect — Wikipedia](https://en.wikipedia.org/wiki/Overjustification_effect)
- [Overjustification Effect — Structural Learning](https://www.structural-learning.com/post/overjustification-effect)
- [Deci, Koestner & Ryan 1999 meta-analysis](https://home.ubalt.edu/tmitch/642/articles%20syllabus/Deci%20Koestner%20Ryan%20meta%20IM%20psy%20bull%2099.pdf)
- [Flow (psychology) — Wikipedia](https://en.wikipedia.org/wiki/Flow_(psychology))
- [Flow Theory: A Learning Professional's Guide — Growth Engineering](https://www.growthengineering.co.uk/flow-theory/)
- [Flow State in Learning — Structural Learning](https://www.structural-learning.com/post/flow-state)
- [Variable Rewards: Want To Hook Users — Nir and Far](https://www.nirandfar.com/want-to-hook-your-users-drive-them-crazy/)
- [The Hook Model of Behavioral Design — Mindtools](https://www.mindtools.com/aapqtdb/the-hook-model-of-behavioral-design/)
- [Hooked on Loot Boxes — Derek Mei / Medium](https://medium.com/behavior-design/hooked-on-loot-boxes-how-design-gets-us-addicted-79c45faebc05)
- [Leaderboards in Education systematic review — Li, JCAL 2024](https://onlinelibrary.wiley.com/doi/10.1111/jcal.13077?af=R)
- [Leaderboards That Motivate (Not Demotivate) — Yu-kai Chou](https://yukaichou.com/advanced-gamification/how-to-design-effective-leaderboards-boosting-motivation-and-engagement/)
- [Leaderboard positions & motivation — Emerald Insight](https://www.emerald.com/insight/content/doi/10.1108/intr-12-2021-0897/full/html)
- [Longitudinal quasi-experiment of leaderboard effectiveness — ScienceDirect](https://www.sciencedirect.com/science/article/abs/pii/S1041608024001651)
- [FTC Dark Patterns filing — Fair Play for Kids](https://fairplayforkids.org/wp-content/uploads/2021/05/darkpatterns.pdf)
- [The Dark Side of Fun: Dark Patterns in Early Childhood Mobile Gaming](https://papers.academic-conferences.org/index.php/ecgbl/article/download/1656/1700/6569)
- [Dark Patterns: Deceptive Design — Congressional Research Service](https://www.congress.gov/crs-product/IF12246)
