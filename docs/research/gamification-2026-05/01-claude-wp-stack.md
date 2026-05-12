# WordPress Gamification Trigger Inventory for Paper LMS

*Claude research agent, 2026-05-12. Focus: MyCred, GamiPress, BadgeOS, LearnDash, BuddyBoss/BuddyPress.*

A consolidated trigger taxonomy distilled from MyCred, GamiPress, BadgeOS, LearnDash, and BuddyBoss/BuddyPress — designed to be implemented as toggleable triggers in a Canvas-like LMS, with FERPA-aware defaults for K-12.

---

## 1. MyCred — Built-in Hooks and Add-ons

MyCred's architecture uses "hooks" (drag-into-sidebar widgets in the admin) and "log references" (canonical event slugs written to the points log). Each hook exposes points value, optional log template, and — for the time-bound hooks — a frequency limiter ("unlimited", "once per day", "once per week", "once total"). ([codex.mycred.me hooks](https://codex.mycred.me/category/hooks/), [codex.mycred.me log references](https://codex.mycred.me/chapter-vi/log-references/))

| Trigger | Fires on | Knobs | Scope |
|---|---|---|---|
| `registration` | New user signup | Points, one-time only | per-user |
| `logging_in` | Successful login | Points, frequency limit (once/day/week/total) | per-user |
| `site_visit` | First page-view per session | Points, daily cap | per-user |
| `view_content` | Viewing a post/page/CPT | Points, per content type, daily cap | per-user, per-post |
| `view_content_author` | Author's content viewed | Points to author | per-author |
| `publishing_content` | Post/page/CPT published | Points, per post type, max-per-day | per-user |
| `approved_comment` / `unapproved_comment` / `spam_comment` / `deleted_comment` | Comment lifecycle | Points (positive or negative) | per-user, per-post |
| `link_click` | Click on `[mycred_link]` shortcode URL | Points, max-per-link | per-user, per-link |
| `watching_video` | YouTube/Vimeo watched to threshold | Points, % watched, max-per-day | per-user, per-video |
| `visitor_referral` / `signup_referral` | Referral URL drove visit/signup | Points, daily cap | per-referrer |
| `anniversary` | Membership year anniversary | Points per year | per-user |
| `transfer` | User→user point transfer | Min/max amount, cooldown, daily limit | per-user |
| `interest` | Banking add-on compound interest | Rate, schedule | per-user |
| `recurring` | Scheduled payout (subscription) | Period, amount | per-user |
| `badge_reward` | Badge awarded | Points piggyback | per-user |
| `coupon` | Coupon redemption | Points credit | per-user |
| `buy_creds_with_*` (paypal, bitpay, skrill, bank, netbilling, zombaio) | Gateway top-up | Exchange rate, fees | per-user |
| `woocommerce_payment` / `_refund` | Order paid/refunded | Points per $ | per-user |
| `store_sale` / `store_sale_refund` / `marketpress_payment` / `wpecom_payment` / `event_payment` / `ticket_purchase` | Commerce gateways | Per-product overrides | per-user |
| `buy_content` / `sell_content` | Paywalled post unlock | Author share, price | per-user |

**MyCred LearnDash add-on hooks** ([codex.mycred.me/chapter-iii/freebies/mycred-learndash](https://codex.mycred.me/chapter-iii/freebies/mycred-learndash/)):

| Trigger | Fires on | Knobs | Scope |
|---|---|---|---|
| `learndash_course_enrolled` | Course enrollment | Points, per-course override | per-course |
| `learndash_course_completed` | Course completion | Points, per-course override | per-course |
| `learndash_lesson_completed` | Lesson completion | Points, per-lesson override | per-lesson |
| `learndash_topic_completed` | Topic completion | Points | per-topic |
| `learndash_quiz_completed` | Quiz attempt finished | Points, per-quiz override | per-quiz |
| `learndash_quiz_failed` (deduction) | Quiz failed | Negative points | per-quiz |
| `learndash_quiz_max_grade` | 100% score | Bonus points | per-quiz |
| `learndash_quiz_grade_range` | Score within band | Tiered points by % | per-quiz |
| `learndash_assignment_upload` | Assignment uploaded | Points | per-lesson |
| `learndash_assignment_approved` | Teacher approves assignment | Points | per-lesson |

**Per-correct-answer / adaptive math pattern** — MyCred LearnDash exposes a per-question points field with decimal + negative support, so a randomized math word-problem question pool awards coins per correct answer rather than per-quiz. ([codex.mycred.me LearnDash Quizzes](https://codex.mycred.me/chapter-iii/freebies/mycred-learndash/mycred-learndash-quizzes/))

**BuddyPress hooks** (MyCred ships these natively): `new_profile_update`, `upload_avatar`, `upload_cover`, `new_friendship`, `ended_friendship`, `new_message`, `sending_gift`, `creation_of_new_group`, `joining_group`, `leaving_group`, `new_group_forum_topic`, `new_group_forum_post`, `fave_activity`, `new_buddypress_gallery`.

**Ranks & leaderboards** — MyCred Ranks support published ranks (auto-promote by point threshold) or manual ranks; leaderboards can be site-wide, per-rank, time-windowed (today/this-week/this-month/all-time), or **relative** ("show me top 5 users within ±N rank of current viewer") — the relative leaderboard is the FERPA-friendly mode because it never exposes full class lists.

---

## 2. GamiPress — Triggers + Requirements Engine

GamiPress's core abstraction is **Events × Requirements**. An event is the firing trigger; a requirement composes one or more events with **All / Any / Number-of (N out of M)** logic. ([gamipress.com/docs events](https://gamipress.com/docs/getting-started/events/))

**Core events**: registration, login, daily visit, post visit, comment posted, post published, post deleted, role added/removed, user-meta updated (with-value match), post-meta updated, achievement unlocked/revoked, points earned/spent/reached-balance, rank reached/revoked.

**LearnDash add-on** ([gamipress.com/add-ons/learndash-integration](https://gamipress.com/add-ons/learndash-integration/)) — every event exists in **any / specific / by-category / by-tag / by-course** variants:
- Enroll in any course / specific course / category / tag
- Complete any course / specific course / category / tag
- Complete any lesson / specific lesson / mark-incomplete
- Complete any topic / specific topic
- Complete any quiz / specific quiz, plus minimum-%, maximum-%, range-%
- Submit essay for a quiz
- Upload assignment, approve assignment
- Join LearnDash Group

GamiPress also ships parallel add-ons for **LearnPress** and **Tutor LMS** with mirrored shapes (lesson_complete, quiz_complete, course_complete, certificate_earned).

**BuddyPress / BuddyBoss add-on** ([gamipress.com/add-ons/buddypress-integration](https://gamipress.com/add-ons/buddypress-integration/)): account activation, profile-type assigned, profile updated, send/accept/reject/remove friendship + the inverse "get a friendship X" events, send/reply private message, publish activity post, reply to activity post, favorite activity, publish activity in group, create/delete group, join/leave group, request to join private group, get accepted, invite to group, get promoted to mod/admin, promote another. BuddyBoss Plus advertises **90+ configurable triggers** in its bundled gamification stack ([buddyboss.com/blog/buddyboss-gamification-setup](https://buddyboss.com/blog/buddyboss-gamification-setup-for-communities/)).

**WooCommerce add-on**: new product purchase, specific-product purchase, category/tag purchase, refund, lifetime-value threshold, subscription renewal.

**Configuration knobs (every requirement)**: count (do this N times), limit (max-per-day/week/month/year/lifetime), points value, points type, cooldown window, prerequisites (must hold badge X first), restricted-to (role / group / course), and a **required-step description** override for UI.

**Achievement types** are user-defined CPTs — you can create unlimited types (Badge, Trophy, Medal, Streak Token, Mastery Pin, etc.). **Rank types** are tiered with auto-promotion at point thresholds.

---

## 3. BadgeOS — The OG, Still Worth Stealing From

BadgeOS introduced the **Required Steps Manager** — a visual builder where any composite of triggers, point thresholds, or "earn this other badge first" rules forms an achievement. It is Mozilla OBI / Open Badges compatible and has a Credly bridge (now sunsetting in favor of native Open Badges 3.0). ([badgeos.org/docs/tutorials/open-badge-compliance](https://badgeos.org/docs/tutorials/open-badge-compliance/), [GitHub opencredit/badgeos](https://github.com/opencredit/badgeos))

**Distinctive bits Paper LMS should steal**:
- **Achievement-as-step** — earning Badge A can be a literal step inside Badge B (Canvas-style learning pathway).
- **Point-threshold steps** — "earn 500 XP total" as a step alongside event-based steps.
- **Hierarchical rank types** (Bronze→Silver→Gold inside an "Algebra" rank type).
- **BadgeStack add-on** scaffolds a starter pack of achievement types + sample content — useful as a "first-run wizard" for new teachers.

---

## 4. LearnDash Native Hooks (Source of Truth for Events)

These are the canonical PHP action hooks gamification plugins listen to ([developers.learndash.com/hooks](https://developers.learndash.com/hooks/)):

- `learndash_course_completed` — `$user, $course_id, $course_progress`
- `learndash_lesson_completed` — `$lesson_data`
- `learndash_topic_completed` — `$topic_data`
- `learndash_quiz_completed` — `$quizdata, $user` (includes pass/fail flag, %, points, time)
- `learndash_quiz_submitted`
- `learndash_essay_submitted`
- `learndash_assignment_uploaded`
- `learndash_assignment_approved`
- `learndash_certificate_created`
- `learndash_group_enrolled` / `learndash_group_completed`
- `learndash_update_course_access` (drip unlock fired)

**Per-question correctness** is exposed via the WPProQuiz layer LearnDash uses — each question has a points field (decimal + negative allowed), so MyCred / GamiPress can award coins per correct answer for the adaptive math word-problem pattern you ran pre-AI.

---

## 5. BuddyBoss / BuddyPress Social Triggers (Beyond GamiPress)

Native BuddyBoss Plus gamification adds: profile-completion-% milestones (50/75/100%), avatar uploaded, cover photo uploaded, **mention received**, **reaction received** (like/love/etc. on activity), reply on your activity, forum topic created, forum reply created, forum topic favorited, message sent, message replied, media uploaded (photo/video/audio/document), document downloaded, gallery created, **invite accepted** (your invite landed a signup), and **member-type changed**.

---

## 6. FERPA-Safe Badging — What Makes a Badge Implementation Compliant

FERPA-safe badging is not the badge format itself — it is **what you put in the badge and where it travels**.

- **Open Badges 2.0** bakes recipient identifier (typically hashed email + salt) into the badge JSON. The hash is reversible by anyone who guesses the email + salt, so the badge JSON is functionally PII unless explicitly consented to. ([openbadges.org/about/faq](https://openbadges.org/about/faq), [issuebadge.com guides](https://issuebadge.com/guides/open-badges-standard))
- **Open Badges 3.0** moves to W3C Verifiable Credentials and DIDs — recipient is a DID, which is pseudonymous unless the student chooses to bind it to an email/wallet. This is the FERPA-preferable shape. ([imsglobal.org spec OB v3p0](https://www.imsglobal.org/spec/ob/v3p0/cert), [certifier.io blog open-badges-3-0](https://certifier.io/blog/open-badges-3-0))
- **FERPA-safe defaults for K-12**: badge lives in the LMS only (internal `badge_award` event), no third-party issuer endpoint reached without parent/eligible-student consent, no email in metadata, no grades or individual test scores in the badge criteria, public-facing badge display **opt-in per student**, no leaderboards exposing surnames outside the class section, no Credly/external sharing without explicit release. BadgeOS docs explicitly call this out as the schools' responsibility ([badgeos.org/docs/tutorials/open-badge-compliance](https://badgeos.org/docs/tutorials/open-badge-compliance/)).

**MyCred's relative leaderboard** (top 5 within ±N ranks of viewer) is the design pattern Paper LMS should adopt as the K-12 default — it never enumerates a full class list and never shows a student a teacher's score or vice versa.

---

## 7. Unified Trigger Taxonomy — Paper LMS

Below: ~95 canonical triggers, grouped, each labeled with **[scope]** and flagged: **(T)** teacher-grantable, **(C)** needs cooldown to prevent farming, **(F)** typically OFF in K-12 for FERPA/compliance.

### A. Learning Progress (12)
1. `enrollment.course` — student enrolls in course [per-course]
2. `progress.lesson_started` — first view of a lesson [per-lesson]
3. `progress.lesson_completed` — lesson marked complete [per-lesson]
4. `progress.topic_completed` — sub-lesson topic complete [per-topic]
5. `progress.module_completed` — Canvas module fully done [per-module]
6. `progress.course_completed` — course progression 100% [per-course]
7. `progress.course_certificate_earned` — certificate generated [per-course]
8. `progress.outcome_mastered` — Canvas learning-outcome mastered [per-outcome]
9. `progress.drip_unlocked` — schedule-released content opened [per-item, C]
10. `progress.prerequisite_cleared` — gating prerequisite satisfied [per-item]
11. `progress.path_milestone` — N% through learning path [per-path]
12. `progress.first_complete_of_day` — first item completed today [per-user, C]

### B. Assessment Mastery (15)
13. `quiz.attempted` — attempt started [per-quiz, C]
14. `quiz.completed` — attempt submitted [per-quiz]
15. `quiz.passed` — at/above passing % [per-quiz]
16. `quiz.failed` — below passing % (deduction-eligible) [per-quiz]
17. `quiz.perfect_score` — 100% [per-quiz]
18. `quiz.grade_range` — score within band (tiered XP) [per-quiz]
19. `quiz.question_correct` — per-question award (adaptive math) [per-question, C]
20. `quiz.question_incorrect` — optional deduction [per-question]
21. `quiz.improved_on_retake` — higher than previous attempt [per-quiz]
22. `quiz.first_try_pass` — passed without retake [per-quiz]
23. `quiz.essay_submitted` [per-quiz]
24. `assignment.submitted` [per-assignment]
25. `assignment.graded` — teacher grade posted [per-assignment]
26. `assignment.on_time` — submitted before due date [per-assignment]
27. `rubric.criterion_mastery` — full points on a rubric row [per-rubric, T]

### C. Time / Streak (10)
28. `time.daily_login` — first login of the day [per-user, C]
29. `time.login_streak_N` — N consecutive days (3/7/14/30) [per-user]
30. `time.study_streak_N` — N consecutive days with progress event [per-user]
31. `time.weekly_active` — active ≥X days in week [per-user, C]
32. `time.session_duration` — focused time threshold (Pomodoro) [per-user, C]
33. `time.early_submitter` — assignment submitted >48h early [per-assignment]
34. `time.before_due` — submitted within N hours of due [per-assignment]
35. `time.late_recovery` — turned in late but completed [per-assignment]
36. `time.anniversary` — N years on platform [per-user]
37. `time.semester_complete` — finished term active [per-user]

### D. Social (15) — most are **(F)** by default in K-12
38. `social.friend_request_sent` [per-user, F]
39. `social.friend_accepted` [per-user, F]
40. `social.group_joined` (study group / club) [per-group]
41. `social.group_created` [per-user, T-approved]
42. `social.group_promoted_mod` [per-group, T]
43. `social.activity_published` (status update) [per-user, F]
44. `social.activity_reply` [per-user, F, C]
45. `social.reaction_given` [per-user, C]
46. `social.reaction_received` [per-user, F]
47. `social.mention_received` [per-user, F]
48. `social.message_sent` [per-user, C, F]
49. `social.invite_landed` — invite drove signup [per-user, F]
50. `social.profile_complete_50` / `_75` / `_100` [per-user]
51. `social.avatar_uploaded` [per-user, F-by-default in K-12]
52. `social.peer_review_submitted` [per-assignment]

### E. Content Creation (12)
53. `content.discussion_post` — top-level [per-thread, C]
54. `content.discussion_reply` [per-thread, C]
55. `content.discussion_quality_reply` — teacher-flagged "good answer" [per-thread, T]
56. `content.first_post_in_thread` [per-thread]
57. `content.media_upload` (photo/video/audio/doc) [per-user, C]
58. `content.gallery_created` [per-user]
59. `content.notes_published` (shared study notes) [per-user]
60. `content.flashcard_set_created` [per-user]
61. `content.wiki_edit` (course wiki contribution) [per-page, C]
62. `content.question_asked` — public Q to instructor [per-course, C]
63. `content.answer_accepted` — student answer accepted [per-question, T]
64. `content.tutorial_video_published` [per-user, T-approved]

### F. Engagement Depth (10)
65. `engage.video_watched_threshold` — ≥80% of lecture video [per-video, C]
66. `engage.transcript_opened` [per-video]
67. `engage.captions_enabled` (accessibility nudge) [per-user]
68. `engage.resource_downloaded` [per-resource, C]
69. `engage.external_link_clicked` (curated resources) [per-link, C]
70. `engage.calendar_event_attended` (synchronous class join) [per-event]
71. `engage.office_hours_attended` [per-event]
72. `engage.poll_voted` [per-poll]
73. `engage.survey_completed` [per-survey]
74. `engage.feedback_submitted` (course feedback) [per-course]

### G. Admin / Teacher-Granted (10) — all **(T)**
75. `teacher.kudos` — manual XP award with note [per-user, T]
76. `teacher.badge_award` — manual badge issuance [per-user, T]
77. `teacher.weekly_mvp` — class-vote or teacher-pick [per-section, T]
78. `teacher.helpful_to_peer` — teacher-observed helpfulness [per-user, T]
79. `teacher.improvement_award` — grade-trajectory bonus [per-user, T]
80. `teacher.attendance_bonus` — synchronous attendance [per-event, T]
81. `teacher.behavior_correction` — negative-points (use with care) [per-user, T, F]
82. `admin.role_assigned` (e.g. "Teaching Assistant") [per-user, T]
83. `admin.bulk_award` — section-wide award [per-section, T]
84. `admin.compliance_acknowledged` — student signed AUP / consent [per-user]

### H. Economy / Spend (8) — for Paper LMS coin store
85. `economy.coin_earned` (rollup of all positive XP) [per-user]
86. `economy.coin_spent` (store purchase) [per-user]
87. `economy.transfer_sent` (peer gift, **F** in K-12, off by default) [per-user, F, C]
88. `economy.transfer_received` [per-user, F]
89. `economy.streak_freeze_used` (Duolingo-style) [per-user, C]
90. `economy.theme_unlocked` [per-user]
91. `economy.avatar_item_unlocked` [per-user]
92. `economy.charity_donation` — converted coins to teacher-defined cause [per-user, T]

### I. Rank / Meta (3)
93. `meta.rank_promoted` [per-user]
94. `meta.achievement_unlocked` (badge-earned meta-event) [per-user]
95. `meta.leaderboard_position_changed` (cohort, opt-in display) [per-user, F]

---

## 8. Flags Summary

- **Default-OFF in K-12 (FERPA / age-appropriateness)**: 38–49 (most social), 51, 52, 81, 87, 88, 95, plus any public site-wide leaderboard. Use MyCred-style **relative leaderboards** as the on-by-default mode.
- **Teacher-grantable (not system-fired)**: 14 triggers — all of the Admin/Teacher group (75–84) plus discussion-quality (55), answer-accepted (63), rubric-criterion (27), peer-review (52 needs both modes).
- **Needs cooldown / max-per-day** (anti-farming): 9, 12, 13, 19, 28, 31, 32, 44, 45, 48, 53, 54, 57, 61, 62, 65, 68, 69, 87, 89. Default cooldowns: 60s (rapid actions like reactions), 5min (discussion posts), 24h (login/streak), once-per-content (video-watched, lesson-completed).

---

## 9. Implementation Notes for Paper LMS

1. **Mirror GamiPress's Events × Requirements split** — events are immutable firings; requirements are user-composable rules using All / Any / N-of-M. This gives teachers + admins the same toolkit without code.
2. **Mirror MyCred's points-types** — Paper LMS should support multiple parallel currencies (XP, coins, mastery-tokens, behavior-bucks) so K-12 teachers can run a Class Dojo replacement on top of the academic XP economy.
3. **Mirror BadgeOS's "badge-as-step"** — a learning pathway is just a badge whose required steps are other badges.
4. **Open Badges 3.0 export, opt-in only** — internal `badge_award` is the FERPA-safe default; the OB3 VC export is an explicit student/parent action, never automatic.
5. **Per-question coin awards** — wire the question-correctness signal from the quiz engine directly into the XP economy so randomized math problem banks award per-correct-answer (the workflow you ran pre-AI).
6. **Toggleable at three levels** — site (district), course (teacher), section (class) — so a 5th-grade teacher can turn off social triggers their middle-school colleague leaves on.

---

## Sources

- [myCred Hooks category — myCred Codex](https://codex.mycred.me/category/hooks/)
- [myCred Log References — myCred Codex](https://codex.mycred.me/chapter-vi/log-references/)
- [myCred Hook API — myCred Codex](https://codex.mycred.me/chapter-v/hook-api/)
- [myCred LearnDash add-on — myCred Codex](https://codex.mycred.me/chapter-iii/freebies/mycred-learndash/)
- [myCred LearnDash Quizzes — myCred Codex](https://codex.mycred.me/chapter-iii/freebies/mycred-learndash/mycred-learndash-quizzes/)
- [myCred Referrals hook — myCred Codex](https://codex.mycred.me/hooks/referrals/)
- [myCred Daily Login Rewards — myCred Codex](https://codex.mycred.me/chapter-iv/enhancements/mycred-daily-login-rewards/)
- [GamiPress Events — gamipress.com](https://gamipress.com/docs/getting-started/events/)
- [GamiPress LearnDash integration](https://gamipress.com/add-ons/learndash-integration/)
- [GamiPress BuddyPress integration](https://gamipress.com/add-ons/buddypress-integration/)
- [GamiPress BuddyBoss integration](https://gamipress.com/add-ons/buddyboss-integration/)
- [GamiPress WooCommerce integration](https://gamipress.com/add-ons/woocommerce-integration/)
- [BuddyBoss Gamification Setup blog](https://buddyboss.com/blog/buddyboss-gamification-setup-for-communities/)
- [BadgeOS Open Badge Compliance docs](https://badgeos.org/docs/tutorials/open-badge-compliance/)
- [BadgeOS Features](https://badgeos.org/about/features/)
- [BadgeOS GitHub (opencredit/badgeos)](https://github.com/opencredit/badgeos)
- [LearnDash Developer Hooks index](https://developers.learndash.com/hooks/)
- [learndash_course_completed hook](https://developers.learndash.com/hook/learndash_course_completed/)
- [learndash_lesson_completed hook](https://developers.learndash.com/hook/learndash_lesson_completed/)
- [learndash_topic_completed hook](https://developers.learndash.com/hook/learndash_topic_completed/)
- [learndash_quiz_completed hook](https://developers.learndash.com/hook/learndash_quiz_completed/)
- [IMS Open Badges 3.0 spec](https://www.imsglobal.org/spec/ob/v3p0/cert)
- [Open Badges 3.0 explainer — Certifier](https://certifier.io/blog/open-badges-3-0)
- [IMS Open Badges FAQ](https://openbadges.org/about/faq)
- [IssueBadge Open Badges 2.0 & 3.0 guide](https://issuebadge.com/guides/open-badges-standard)
- [US DoE Student Privacy / FERPA](https://studentprivacy.ed.gov/ferpa)
