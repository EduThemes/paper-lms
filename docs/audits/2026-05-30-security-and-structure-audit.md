# Paper LMS — Security & Code-Organization Audit Report

**Audit date:** 2026-05-30
**Target:** Paper LMS — multi-tenant Go (Fiber) + React Learning Management System
**Branch/commit audited:** `main` @ `549edad` (latest migration 000062)
**Classification:** Public-facing-quality audit, read-only methodology
**Lead author:** Security & architecture review panel

---

## 1. Scope & Methodology

### Scope
The audit covered the full Go backend (`internal/`, `cmd/`), the React frontend (`web/src/`), the GraphQL surface, the SQL migration chain, the SSO/SAML/OIDC/LDAP/CAS authentication subsystem, the multi-tenant authorization model, the file-handling and storage backends (local + S3), the gamification economy, the FERPA/COPPA compliance subsystems, dependency posture, CI/CD configuration, and overall code organization. Production deployment configuration (`deployments/docker/`, `docker-compose.prod.yml`) was reviewed for hardening posture.

### Methodology
A read-only, multi-agent fan-out process was used:

- **17 security finder dimensions** — tenant isolation, authz/RBAC, authentication/session, crypto/secrets, injection (SQL + XSS), SSRF/outbound, DoS/resource exhaustion, GraphQL, file handling, IDOR/object-reference, secrets-in-repo, dependency/vuln, headers/CORS/CSRF, SAML/SSO, business logic, frontend auth, and SQL injection.
- **Loop-until-dry** scanning, up to 3 rounds per dimension.
- **Severity-triaged adversarial verification.** Every **Critical** and **High** finding was put through a **3-lens skeptic panel** (3 independent confirmers; votes recorded as `conf`/`ref`). Every **Medium**, **Low**, and **Info** finding received a **single validator** pass. There are **no silent severity caps** — medium/low items were verified, not dropped.
- A **completeness critic** identified unreviewed attack surface (Section 5).
- **8 organization/structure reviewers** covered layering, god files, dead code/duplication, naming, package boundaries, testing, build/CI, and public-repo readiness.

Verification notation in each finding: `3-lens (conf N/ref M)` means N confirmers agreed and M dissented; `1-vote` means single-validator confirmed.

---

## 2. Executive Summary

Paper LMS has a mature, defended core for the threat classes it has historically focused on — SQL injection is comprehensively closed (verified twice, end to end), the rich-content HTML sanitization pipeline is largely consistent, secrets are not committed to the repository, and the local-login session path is correctly hardened. However, the audit surfaced a **systemic and repeated failure of multi-tenant object-level authorization**: a large family of endpoints resolve resources by primary-key ID without tying them to the URL's parent course/account or to the caller's tenant. This single root-cause pattern produces cross-tenant IDORs across SIS exports, grades, rubric assessments, accommodations (disability/IEP data), enrollment terms, grading periods, feature flags, audit logs, and the GraphQL surface. The most severe finding is an **unauthenticated-to-full-deployment PII export** (every user's name/email/login across every tenant) reachable by any account admin. Secondary high-impact clusters are **SSRF via redirect-following HTTP clients**, **session cookies minted without the `Secure` flag on every federated/MFA/passkey path**, **SAML assertion-validation gaps** (Conditions-less assertions bypass audience/expiry; no `InResponseTo` binding), and **stored XSS** through QTI/IMSCC import paths and unsanitized portfolio/collaboration URLs.

The GraphQL endpoint is a concentrated liability: it applies tenant scope inconsistently, performs zero enrollment/role authorization, has no query-depth/complexity/rate limits, and leaks raw ORM errors. The code-organization review found the architecture fundamentally sound but suffering from three oversized flat packages, several fully-built-but-unmounted feature stacks, and — most importantly for release readiness — **no security scanning in CI whatsoever** (no `govulncheck`, no SAST, no dependency or container scanning), production Docker images running as **root**, and advisory-only linting.

### Confirmed security findings by severity

| Severity | Count |
|----------|-------|
| Critical | 5 |
| High | 38 |
| Medium | 24 |
| Low | 24 |
| Info | 11 |
| **Total** | **102** |

> **On the counts.** The table above reports the **102 distinct issues** written up in this report. The fan-out actually produced **164 raw findings that survived adversarial verification** (Critical 5 · High 52 · Medium 48 · Low 42 · Info 17); the difference is cross-dimension overlap — the same sink was independently surfaced by more than one finder (e.g. the GraphQL `allCourses`/SpeedGrader/rubric/calendar IDORs hit both the tenant-isolation and GraphQL finders) and is consolidated here under one entry. The complete, un-consolidated register of all 164 is in **Appendix B (Section 9)** for traceability; cross-references are noted inline below. The 5 Critical and 52 High findings each cleared a 3-vote adversarial panel; Medium/Low/Info cleared a single validator and should be treated as a high-quality lead list to triage rather than confirmed-exploitable.

---

## 3. Confirmed Security Findings

### CRITICAL

---

#### SEC-001 — GraphQL `allCourses` returns every tenant's courses (cross-tenant enumeration)
**Category:** Broken authorization / multi-tenancy (BOLA)
**Location:** `internal/graphql/resolver.go:142` → `internal/service/course_service.go:76-78` → `internal/repository/postgres/course.go:51-58`

**Description.** The `allCourses` root resolver calls `r.courseService.List(ctx, params)`, which hardcodes `accountID=0`: `return s.courseRepo.List(ctx, 0, params)`. In the repo, the tenant filter is only applied `if accountID != 0`, so `0` yields `WHERE workflow_state != 'deleted'` with no tenant predicate. The GraphQL handler threads the real tenant into ctx via `WithAccountID`, but this resolver never reads `AccountIDFromContext(ctx)`. The `/graphql` route (`router.go:628`) carries only `Protected()` + CSRF — no role gate.

**Impact.** Any authenticated user of any tenant (including a student) can post `{ allCourses(perPage: 100000) { id name course_code account_id } }` and enumerate every course in the entire multi-tenant database. The `account_id` field directly reveals tenant partitioning. The same unscoped sink is reachable via REST `GET /api/v1/courses?scope=all` (SEC-006).

**Recommendation.** Add an `accountID` parameter to `CourseService.List`, pass `AccountIDFromContext(ctx)` from `resolveAllCourses` and `callerAccountID(c)` from the REST handler, and reject `accountID==0` from request-facing paths. Audit every `resolveAll*` resolver for the same omission.

**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-002 — Any enrolled student can read any classmate's submission, score, grade, and attachments (`GetSubmission`)
**Category:** IDOR / Broken Object-Level Authorization
**Location:** `internal/api/v1/handlers/submissions.go:170-192`

**Description.** `GET /courses/:course_id/assignments/:assignment_id/submissions/:user_id` is behind the `enrolled` middleware (`router.go:334`), which only requires any active enrollment in the course. The handler reads `:user_id` from the path and calls `GetByAssignmentAndUser(assignmentID, userID, callerAccountID(c))` with no comparison of `:user_id` to the caller, no teacher/TA check, and no observer-link check. The sibling quiz-submission handler (`quiz_submissions.go:147-163`) demonstrates the correct guard, confirming this is an omission.

**Impact.** A student can read every other student's submission body, file attachments, numeric score, letter grade, grader_id, and excused/late flags by incrementing `:user_id`. Sequential integer IDs make full-roster enumeration trivial — a FERPA-grade disclosure of graded work.

**Recommendation.** After loading, if `submission.UserID != callerUserID`, require Teacher/TA enrollment (from Locals) or a verified observer link, else `responses.NotFound`. Also tie `:assignment_id` to `:course_id`.

**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-003 — `ListCourseSubmissions` returns every student's grades when `user_id` is omitted
**Category:** IDOR / Broken Object-Level Authorization
**Location:** `internal/api/v1/handlers/submissions.go:86-136`

**Description.** `GET /courses/:course_id/submissions` (`enrolled`) has an observer/self check **only when** the optional `user_id` query param is present. With the param absent, `filterUserID` stays `0`, the per-row filter (`if filterUserID > 0 && s.UserID != filterUserID`) becomes a no-op, and the handler returns the full `BulkListByCourse` result (all users, all assignments).

**Impact.** Any enrolled student can call `GET /courses/:course_id/submissions` with no `user_id` and receive every classmate's submission body, score, and grade for the whole course in one paginated call (PerPage up to 10000). This is faster, broader roster-wide exfiltration than SEC-002.

**Recommendation.** If the caller is not Teacher/TA/admin, force `filterUserID` to the caller's own user_id (or a linked observed student). Default-deny the roster-wide view.

**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-004 — `ListSubmissions` exposes all submissions for any assignment with no role check, parent-tie, or tenant scope
**Category:** IDOR / Broken Object-Level Authorization
**Location:** `internal/api/v1/handlers/submissions.go:138-168`

**Description.** `GET /courses/:course_id/assignments/:assignment_id/submissions` (`enrolled`) calls `ListByAssignment(assignmentID, params)`. `ListByAssignmentID` (`submission.go:85-97`) filters **only** by `assignment_id` — no user filter, no `account_id` tenant filter, and no tie between `:assignment_id` and `:course_id`. The `enrolled` guard only proves enrollment in `:course_id`.

**Impact.** A student enrolled in any one course can read all submissions (body/score/grade) for **any** assignment in the system by supplying its `assignment_id`, including assignments in other courses and — because no `account_id` is applied — **other tenants**. Cross-student and cross-tenant grade disclosure.

**Recommendation.** Restrict to instructor/TA/admin (read `enrollment_type` from Locals as the quiz path does), verify `assignment.CourseID == :course_id`, and tenant-scope the query.

**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-005 — Full-deployment PII export via SIS CSV endpoints (account_id discarded, query has no tenant filter)
**Category:** Broken Access Control / Cross-Tenant IDOR
**Location:** `internal/api/v1/handlers/sis_imports.go:141,157,173,189` → `internal/service/sis_import_service.go:576,614,703`

**Description.** The four SIS export routes are admin-gated under `/accounts/:account_id/sis_exports/*`. `RequireAdmin` only verifies the caller is an admin of **their own** tenant (it compares `user.AccountID` to the JWT-derived `account_id`, never the URL `:account_id`). The handlers then parse-and-discard the URL account id (`_, err := c.ParamsInt("account_id")`) and call `h.sisService.ExportUsersCSV(c.Context())` with no account argument. The service runs `s.db.WithContext(ctx).Find(&users)` with **no `WHERE` clause**, returning every user in the entire deployment. Identical shape for `ExportCoursesCSV` and `ExportEnrollmentsCSV`.

**Impact.** Any account admin of any tenant can `GET /api/v1/accounts/<any>/sis_exports/users.csv` and receive the login_id, name, and email of every user across every tenant, plus full course and enrollment rosters. Complete breach of multi-tenant isolation and a mass FERPA/PII disclosure.

**Recommendation.** Thread `callerAccountID(c)` into the export service methods, add `WHERE account_id = ?` (or the tenant join for courses/enrollments), and reject when the URL `:account_id != callerAccountID(c)` (404 per the existence-leak contract). Stop parse-and-discarding the account id.

**Verification:** 3-lens (conf 3 / ref 0).

---

### HIGH

The High findings cluster into recurring root causes. They are grouped by theme for readability; each retains a stable ID.

#### Theme A — Cross-tenant / cross-course IDOR from missing parent-tie + tenant scope

---

#### SEC-006 — `GET /courses?scope=all` returns all courses across all tenants to any authenticated user
**Category:** Cross-tenant data leak
**Location:** `internal/api/v1/handlers/courses.go` `ListCourses` (`scope=="all"`) + `internal/service/course_service.go:76-78`
**Description.** The route has no admin gate. `?scope=all` calls `CourseService.List` → `courseRepo.List(ctx, 0, params)` (hardcoded `accountID=0`), returning every non-deleted course in the deployment. A handler comment claims an "admin use case" but no admin check exists.
**Impact.** Any authenticated user — including a student — can enumerate course names/codes/metadata for every tenant. REST twin of SEC-001.
**Recommendation.** Thread `callerAccountID(c)` into `CourseService.List`; allow `0` only for super_admin, or gate the `scope=all` branch behind `RequireSuperAdmin`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-007 — Quiz question endpoints lack parent-tie + tenant scope (cross-tenant read/modify/delete)
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/quiz_questions.go:40,63,121,185` + `internal/service/quiz_authoring_service.go:63-73`
**Description.** Routes `/courses/:course_id/quizzes/:quiz_id/questions/:question_id` are gated by `enrolled`/`instructor` on `:course_id` only. Handlers fetch/update/delete purely by `:question_id` via `GetQuestion(ctx, id)` → `questionRepo.FindByID(ctx, id)` — no accountID, no parent-tie. `ListQuestions` is likewise unscoped.
**Impact.** Any enrolled student can read any quiz question across tenants — including `answers` and `correct_comments` (exposed by `quizQuestionToJSON`), leaking correct answers and FERPA content. Any instructor can rewrite/destroy assessment content in any tenant.
**Recommendation.** Resolve `:quiz_id` via `requireQuizInCourse(c, quizID, courseID)` and assert `question.QuizID == :quiz_id` (404 on mismatch), mirroring `quiz_submissions.go`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-008 — Quiz question-group endpoints lack parent-tie + tenant scope
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/quiz_question_groups.go:88,103,150`
**Description.** Same root cause and same `QuizService` as SEC-007. `GetGroup`/`UpdateGroup`/`DeleteGroup` fetch by `:group_id` alone (`FindByID(ctx, id)`); no tie to `:quiz_id`/`:course_id`/tenant.
**Impact.** A teacher in any tenant can read/modify (name, pick_count, points, question_bank_id) or delete quiz groups in any other tenant, corrupting scoring/structure.
**Recommendation.** Add `requireQuizInCourse` + `group.QuizID == :quiz_id` guard.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-009 — Commons Publish snapshots arbitrary cross-tenant resources (data exfiltration)
**Category:** Cross-tenant data exfiltration / IDOR
**Location:** `internal/service/commons_service.go:151-179` (`buildSnapshot`); handler `internal/api/v1/handlers/commons.go:171-209`
**Description.** `POST /courses/:course_id/commons/publish` is `instructor`-gated, but the body's `resource_id` is not tied to `course_id`. `buildSnapshot` fetches each resource with hardcoded `accountID=0` and never checks the resource's parent course (`assignmentRepo.FindByID(ctx, resourceID, 0)`, `quizRepo`, `pageRepo`, `moduleRepo`, `discussionRepo`). The published `SharedContent` is stamped with the attacker's own `course.AccountID`, making it readable back.
**Impact.** A teacher in tenant A can publish `{resource_type:"quiz", resource_id:<quiz in tenant B>}` (or assignment/page/module/discussion) and read tenant B's content — including quiz answers — via `GET /commons/:id`.
**Recommendation.** Fetch each resource scoped to `course.AccountID` and assert `resource.CourseID == course.ID` before bundling; pass `callerAccountID` to `courseRepo.FindByID` at `commons_service.go:88`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-010 — SpeedGrader single-submission endpoint lacks parent-tie/tenant scope (cross-tenant grade/comment leak)
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/speedgrader.go:129-171` → `internal/service/speedgrader_service.go:148-168`
**Description.** `GET /courses/:course_id/assignments/:assignment_id/speedgrader/submissions/:user_id` is `instructor`-gated on `:course_id`. The handler **discards** `:course_id` and calls `GetStudentSubmission(assignmentID, userID)`, which uses `FindByAssignmentAndUser(ctx, assignmentID, userID, 0)` and `ListBySubmissionID(ctx, submission.ID, 0)` — the `accountID=0` escape hatch. The in-code comment asserting upstream tenant verification is false; this handler never loads the parent assignment. Contrast `GetSpeedGraderData` (line 67) which checks `assignment.CourseID != courseID`.
**Impact.** Any teacher/TA of any single course can read any student's submission body, score, and private grading comments for any assignment in any other course/tenant. Cross-tenant FERPA leak.
**Recommendation.** Load the assignment scoped to `callerAccountID(c)`, assert `assignment.CourseID == :course_id`, and thread `callerAccountID(c)` into `GetStudentSubmission` (drop the `0`).
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-011 — Quiz statistics endpoint lacks quiz→course parent-tie and tenant scope
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/quiz_statistics.go:64` → `internal/service/quiz_statistics_service.go:19`
**Description.** `GET /courses/:course_id/quizzes/:quiz_id/statistics` (`instructor`) calls `GetQuiz(ctx, quizID)` → `quizRepo.FindByID(ctx, quizID, 0)`, then fans out to all completed submissions/answers by `quiz_id`. The tenant-scoped `GetQuizScoped` exists but is not used here.
**Impact.** A teacher/TA can retrieve full item-analysis — every completed submission and answer for every student — for any quiz in any other course/tenant. Cross-tenant FERPA leak of quiz performance.
**Recommendation.** Use `GetQuizScoped(ctx, quizID, callerAccountID(c))` and assert `quiz.CourseID == :course_id`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-012 — Discussion entry V2 update has no ownership or parent-tie check
**Category:** Cross-tenant content tampering / IDOR
**Location:** `internal/api/v1/handlers/discussions_v2.go:190` → `internal/service/discussion_v2_service.go:316`
**Description.** `PUT .../entries/:entry_id/v2` is `enrolled`-only. `UpdateEntryWithHistory` fetches `entryRepo.FindByID(ctx, entryID, 0)` and updates it without checking `entry.UserID == userID` or that the entry belongs to `:topic_id`/`:course_id`.
**Impact.** Any user with an active enrollment in any one course can overwrite the message body of any discussion entry in the deployment, across courses and tenants. (Content is HTML-sanitized, so not stored XSS, but integrity/impersonation impact remains.)
**Recommendation.** Fetch scoped to `callerAccountID`, verify the entry's topic belongs to `:course_id`, and require `entry.UserID == userID` OR instructor/admin.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-013 — Rubric assessment Get/Update/List lack parent-tie and tenant scope
**Category:** Cross-tenant student-score read + grade tampering
**Location:** `internal/api/v1/handlers/rubric_assessments.go:82` + `internal/repository/postgres/rubric_assessment.go:23`
**Description.** Routes `/courses/:course_id/rubric_associations/:association_id/rubric_assessments[/:assessment_id]` (`enrolled`/`instructor`) call `GetAssessment(ctx, id)` → `assessRepo.FindByID(ctx, id)` with no accountID and no course/association tie. `ListAssessments`, `GetAssociation`, and `CreateAssessment` share the gap. Only `Delete` was hardened (F-012).
**Impact.** GET: any enrolled user reads any student's per-criterion rubric scores/comments (FERPA), cross-course and cross-tenant. PUT: a teacher overwrites any rubric assessment (student score) in any course/tenant.
**Recommendation.** Thread `callerAccountID(c)` through Get/Update/List/`GetAssociation` (JOIN through `rubric_association → course → account_id`); assert the association/course matches the URL.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-014 — Rubric assessment Create/Update has no parent-tie (cross-tenant grade write + gamification injection)
**Category:** Broken object-level authorization / IDOR
**Location:** `internal/api/v1/handlers/rubric_assessments.go:35-74,90-124` + `internal/repository/postgres/rubric_association.go:23-29`
**Description.** `CreateAssessment` loads the association by `:association_id` via the fully-unscoped `assocRepo.FindByID(id)` and never asserts `assoc.ContextType=="Course" && assoc.ContextID == :course_id`. The service computes `Score = sum(criterion points)` and persists. The created assessment fires the gamification callback, resolving the **victim** tenant's account and emitting `verb=assessed` with `ActorID = attacker-supplied UserID`.
**Impact.** A teacher in one course/tenant can write rubric grades for arbitrary users against rubrics in other courses/tenants, and inject currency/badge events into the victim tenant on behalf of a chosen user.
**Recommendation.** Assert the association/assessment ties to `:course_id`; tenant-scope `FindByID`. Mirror `requireQuizInCourse`/`overrideInCourse`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-015 — Cross-course/cross-tenant IDOR: mastery-path `ReplaceRule`/`DeleteRule` ignore `:course_id`
**Category:** Missing parent-tie (F-013 violation)
**Location:** `internal/api/v1/handlers/mastery_paths.go:137-163` + `internal/service/mastery_path_service.go:141-155`
**Description.** `PUT/DELETE /courses/:course_id/mastery_paths/rules/:rule_id` (`instructor`) ignore `:course_id` and call `ReplaceRule(ctx, ruleID,...)`/`DeleteRule(ctx, ruleID)`, which resolve by id only with no course/account scope and never check `rule.CourseID == :course_id`.
**Impact.** Any instructor in any tenant can overwrite (wipes + recreates scoring ranges) or delete the conditional-release/mastery-path rules of any other course in the deployment.
**Recommendation.** Fetch the rule, reject unless `rule.CourseID == :course_id`, and tenant-scope `FindRuleByID`/`DeleteRule`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-016 — `PostGrades`/`HideGrades` flip grade visibility for any assignment cross-course/cross-tenant
**Category:** Broken Access Control / FERPA disclosure
**Location:** `internal/api/v1/handlers/submissions.go:533-561` + `internal/repository/postgres/submission.go:151-156`
**Description.** `POST /courses/:course_id/assignments/:id/post_grades` and `/hide_grades` are `instructor`-gated on `:course_id` only; the handler reads `:id` directly and calls `PostGradesByAssignment(uint(assignmentID),...)` with no parent-tie. The repo runs `Where("assignment_id = ? AND score IS NOT NULL", assignmentID).Update("posted_at",...)` — fully unscoped.
**Impact.** Any teacher of any throwaway course can pass the guard on their own `:course_id`, then supply an arbitrary assignment ID from any tenant. Setting `posted_at` prematurely reveals muted grades (FERPA); clearing it hides legitimately posted grades (tampering/denial).
**Recommendation.** Verify `assignment.CourseID == :course_id` under the caller's tenant before posting/hiding; scope the bulk update by `account_id`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-017 — Cross-tenant file metadata enumeration via unscoped `/folders/:folder_id/files`
**Category:** Broken Access Control / IDOR
**Location:** `internal/api/v1/handlers/files.go:318-339` (route `router.go:386`; repo `attachment.go:75-93`)
**Description.** `GET /folders/:folder_id/files` has no authorization middleware beyond authentication. `ListFolderFiles` calls `ListFilesByFolder(ctx, folderID, params)` — never loads the folder, never verifies enrollment, never passes `callerAccountID`. The repo query is `WHERE folder_id = ? AND workflow_state != 'deleted'` with no tenant scope.
**Impact.** Any authenticated user (any tenant) can iterate folder IDs and read attachment metadata — `display_name`, `filename`, `content_type`, `size`, `md5`, owner `user_id`, and the download URL — for any folder in any tenant, including FERPA-relevant student documents.
**Recommendation.** Load the folder via `folderRepo.FindByID(ctx, folderID, callerAccountID(c))` (404 on miss), then branch authz by `ContextType` (Course→enrolled, User→owner/admin, Group→member/admin, Account→admin); widen `ListByFolderID` to take `accountID`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-018 — S3 download path bypasses the F-040 Content-Disposition/nosniff inline-XSS control
**Category:** Stored XSS / Content-Type handling
**Location:** `internal/api/v1/handlers/files.go:287-302`; `internal/storage/s3.go:295-310`
**Description.** On the local backend, `DownloadFile` sets `Content-Disposition: attachment` for risky MIME types and relies on the global `nosniff`. For the S3 backend it instead 307-redirects to a presigned URL. `S3Backend.URL` presigns with only `Bucket`+`Key` — no `ResponseContentDisposition`/`ResponseContentType`. S3 then serves the object with the stored object `Content-Type` (set from the attacker-controlled upload header) and no disposition, so the browser renders inline. Upload `Content-Type` is unvalidated for unknown extensions (SEC-040).
**Impact.** Whenever `STORAGE_BACKEND=s3`, the headline F-040 defense is fully bypassed. An attacker uploads `payload.xyz` with `Content-Type: text/html` (or `.svg` via the trusted IMSCC path) and shares `/files/{id}/download` → stored XSS / drive-by execution.
**Recommendation.** Pass `ResponseContentDisposition` and a safe `ResponseContentType` into the presign, or stamp object metadata at `Put` time, or proxy S3 downloads through the app. Store a server-validated Content-Type at upload.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-019 — Cross-tenant IDOR: account-scoped admin endpoints trust URL `:account_id`
**Category:** Cross-tenant IDOR / missing object-tenant binding
**Location:** `internal/api/v1/handlers/developer_keys.go:43-64,99-141`; `auth_providers.go:76-113`
**Description.** `RequireAdmin` confirms only that the caller is an admin of their own tenant; it never compares the URL `:account_id`. `ListDeveloperKeys`/`CreateDeveloperKey` pass `strconv.Atoi(c.Params("account_id"))` to the service unchecked, and `AuthProviderHandler.GetProvider` calls `GetProvider(ctx, id)` with no accountID at all.
**Impact.** An admin in tenant A can enumerate tenant B's developer keys and **mint** a new working OAuth client in tenant B (returning its one-time `client_secret`), and read any tenant's auth-provider config (IdP entity IDs, SAML/CAS/OIDC URLs, LDAP host/bind DN). Secret columns are JSON-hidden, limiting but not eliminating impact.
**Recommendation.** Replace path `:account_id` with `callerAccountID(c)` or call `assertSameTenant(c, pathAccountID)`; thread accountID into `AuthProviderService.GetProvider/Update/Delete`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-020 — Cross-tenant IDOR on enrollment terms (read/update/delete by ID, no tenant tie)
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/enrollment_terms.go:35-210` + `internal/service/enrollment_term_service.go:49,53`
**Description.** `GetTerm`/`UpdateTerm`/`DeleteTerm` discard `:account_id` and resolve by `:id`; `ListTerms`/`CreateTerm` trust the URL `:account_id`. Service methods take no accountID; the handler file contains zero `callerAccountID`/`assertSameTenant` calls. The correct pattern is `AccountHandler` which calls `assertSameTenant`.
**Impact.** Any account admin can list/read/create/rename/delete the enrollment terms of any other tenant. Deleting/altering a term cascades to course-term associations and grading windows.
**Recommendation.** Thread `callerAccountID(c)` through the service/repo and 404 on tenant mismatch.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-021 — Cross-tenant IDOR: grading period groups & periods (hardcoded `accountID=0`)
**Category:** Cross-tenant IDOR
**Location:** `internal/service/grading_period_service.go:32-42,63-76`; handlers `grading_periods.go:52-308`
**Description.** `GetGroup`/`GetPeriod` call `FindByID(ctx, id, 0)` (no comment justifying the escape hatch); handlers never compare the resolved `AccountID` to `callerAccountID(c)`. The repo supports tenant scoping but it is dead on this path. Admin-gated only.
**Impact.** An admin in tenant A can read/rename/re-weight/delete grading period groups and periods owned by tenant B, tampering with grade-weighting and grading windows.
**Recommendation.** Widen `GetGroup`/`GetPeriod` to accept accountID; load-then-assert `AccountID == callerAccountID(c)` for writes (the Wave-F fix already applied to Course/Assignment).
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-022 — Cross-tenant config tampering: account-scoped feature flags trust `:id`
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/feature_flags.go:39-90`
**Description.** `List/Get/Set/DeleteAccountFeature` pass URL `:id` to `flagService.{...}(FeatureContextAccount, uint(id),...)` with no `callerAccountID(c)` comparison; authorization is only `isAdmin(c)` (role-only, tenant-blind).
**Impact.** An admin in tenant A can read and **flip** feature flags for any other tenant (`PUT /accounts/<tenantB>/features/<feature>`), enabling/disabling tenant-wide capabilities.
**Recommendation.** Add `assertSameTenant(c, uint(id))` to each account-scoped feature-flag handler.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-023 — Cross-tenant IDOR: account-scoped outcome proficiency scale read/overwrite/delete
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/outcome_proficiency.go:78-115`
**Description.** `Get/Set/DeleteForAccount` pass URL `:id` to `proficiency.{Get,Set,Reset}(ctx, "Account", uint(id),...)` with no tenant tie. For `contextType=="Account"`, `contextID` *is* the tenant and is attacker-controlled. Course-scoped variants correctly pass `callerAccountID`.
**Impact.** An admin in tenant A can read, replace, or reset the outcome-proficiency/mastery scale of any other tenant, silently re-grading every outcome mastery calculation across that tenant.
**Recommendation.** Add `assertSameTenant(c, uint(id))`; widen the proficiency repo to carry accountID.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-024 — Cross-tenant audit-log read: `GET /accounts/:account_id/audit_log` trusts the URL account id
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/audit.go:200`
**Description.** `GetAccountAuditLog` sets `filter.AccountID = URL :account_id` and calls `GetAuditLog` → `ListByFilter` with no caller-tenant validation. `RequireAdmin` only checks own-tenant admin.
**Impact.** An admin in tenant A can read tenant B's entire audit log — every event with `user_id`, `ip_address`, `user_agent`, and payload.
**Recommendation.** Assert `uint(accountID) == callerAccountID(c)` (or override `filter.AccountID` with it) unless super_admin.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-025 — Cross-tenant IDOR on enrollment-terms admin API (duplicate of root cause; distinct surface)
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/enrollment_terms.go:35-127`
**Description.** Independent finder confirmation of SEC-020: every term handler passes URL `:account_id` / bare term `:id` to the service with no `callerAccountID` tie. Same root cause as SEC-019; this handler was not remediated when developer_keys/auth_providers were.
**Impact.** Cross-tenant list/read/create/rename/delete of enrollment terms.
**Recommendation.** Widen service+repo signatures to take accountID, scope the query, 404 on mismatch.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-026 — Cross-tenant IDOR on grading-period-groups admin API
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/grading_periods.go:52-308`; `grading_period_service.go:44`
**Description.** All group/period handlers trust URL `:account_id`/`:group_id`/`:period_id` with no `callerAccountID` tie (handler-layer twin of SEC-021).
**Impact.** An admin in tenant A can enumerate and mutate grading-period config of tenant B.
**Recommendation.** Add accountID scoping throughout and assert ownership before read/write.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-027 — Cross-tenant IDOR: developer-key / auth-provider account-scoped read & mint (consolidated handler-layer)
*(See SEC-019 — same root cause; retained as a distinct adversarially-verified entry for the developer-key minting and auth-provider read primitives.)*
**Verification:** 3-lens (conf 3 / ref 0).

---

#### Theme B — Authorization design gaps (systemic)

---

#### SEC-028 — Account-scoped admin handlers are systematically tenant-blind (`RequireAdmin` design gap)
**Category:** Broken Access Control / Design Gap
**Location:** `internal/api/v1/middleware/permissions.go:41-79` (cross-referenced with router account routes)
**Description.** `RequireAdmin` verifies `role==admin && user.AccountID == callerAccount` (or super_admin, or root `account_id==1`) but **never reads the URL `:account_id`**. Tenant isolation on `/accounts/:account_id/*` therefore depends entirely on each handler re-deriving `callerAccountID(c)`. Some handlers do (developer_keys, oneroster, custom_roles); many do not (enrollment_terms, grading_periods, sis_imports, account-scope feature_flags/outcome_proficiency). The middleware provides no backstop.
**Impact.** Any account-scoped admin route whose handler forgets the tenant tie becomes a cross-tenant IDOR — the structural root cause of SEC-005, SEC-019..SEC-027.
**Recommendation.** Add a `RequireAccountAdmin(paramName)` guard that parses `:account_id` and enforces it `== callerAccountID` (with documented super_admin/root exception); apply to all `/accounts/:account_id/*` routes so a missing in-handler check fails **closed**.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-029 — `SubmitPeerReview` lets any tenant user forge/overwrite any peer review (no reviewer-ownership check)
**Category:** Broken Access Control / IDOR
**Location:** `internal/api/v1/handlers/peer_reviews.go:77` (route `router.go:800`); service `peer_review_service.go:115-130`
**Description.** `PUT /peer_reviews/:review_id` has no route-level RBAC. The service does `FindByID(reviewID, accountID)` (tenant-scoped) then writes `pr.Score`, `pr.Comments`, `pr.WorkflowState="completed"` — **never** checking `caller == pr.ReviewerID`. Score (a float64 from the body) is also unbounded (no NaN/Inf/range check).
**Impact.** Any authenticated user in the tenant (e.g. a student) can submit/overwrite the score and comments of any peer review by enumerating `review_id`, forging grades and injecting comment text into another reviewer's record. Where scores feed grade aggregation this is direct grade tampering.
**Recommendation.** Add a route guard and in-service check that `caller == pr.ReviewerID` (instructor/admin override only if intended), assert `pr.WorkflowState == "assigned"`, and clamp/validate the score against `points_possible`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-030 — Account-wide/global announcements editable/deletable by any authenticated user (authz skipped when `CourseID` is nil)
**Category:** Broken Access Control
**Location:** `internal/api/v1/handlers/announcements.go:170,240` (routes `router.go:661-662`)
**Description.** `PUT`/`DELETE /announcements/:id` have no route-level RBAC. Authorization is done in-handler **only** when course-scoped: `if announcement.CourseID != nil { RequireCourseInstructor(...) }`. Account-wide/global announcements have `CourseID == nil`, so both handlers skip authz and mutate. The service performs no re-check; `GetAnnouncement(ctx, id)` takes no accountID.
**Impact.** Any low-privilege user can rewrite Title/Message/WorkflowState/priority of, or soft-delete, any account-level or global (deployment-wide via unscoped `GetAnnouncement`) announcement by guessing its ID — defacement/misinformation broadcasts and suppression of admin notices.
**Recommendation.** Add an `else` branch for `CourseID == nil` requiring admin + matching account scope; tenant-scope `GetAnnouncement`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-031 — Cross-tenant IDOR on student accommodations (disability/504/IEP data) — zero tenant scoping in data layer
**Category:** Broken Access Control / Multi-tenancy
**Location:** `internal/repository/postgres/student_accommodation.go:35,47-55`; `accommodation_service.go:67-83`; `accommodations.go:145-262`
**Description.** Every accommodation read/write path takes only `id`/`user_id` and no accountID. Repo queries are `Where("id = ?", id)` / `Where("user_id = ?", userID)` with no `account_id` column referenced. Routes are gated by `admin` / `RequireOwnerOrAdmin` / `selfOrAdmin`, all of whose admin branches are tenant-blind (SEC-032).
**Impact.** An account admin of tenant A can enumerate, read, modify (`status`/`notes`/`plan_type`), and deactivate accommodation records (disability status, 504/IEP plan type, `plan_external_id`, medical-adjacent notes) belonging to students in tenant B — the most sensitive FERPA data the LMS stores. Silent deactivation denies legally-required accommodations.
**Recommendation.** Thread accountID through the entire accommodation stack (repo, service, handler), 404 on tenant mismatch, and verify the target `:user_id` belongs to the caller's tenant on Create.
**Verification:** 3-lens (conf 3 / ref 0). *(A handler-layer twin, SEC-031b, was confirmed at `accommodations.go:172` single-vote.)*

---

#### SEC-032 — `selfOrAdmin`/`RequireAdmin` admin branch is tenant-blind on object-id routes
**Category:** Broken Access Control / Multi-tenancy
**Location:** `internal/api/v1/middleware/permissions.go:200-237,249-268`; `internal/api/v1/handlers/authz.go:26-35`
**Description.** `RequireSelfOrAdmin` falls back to `isAdmin(c, userID)` (role-only, no tenant comparison); `ResourceAuthorizer.isAdmin` returns true for any `role=="admin"` regardless of tenant. Wherever the downstream service/repo is not itself tenant-scoped (accommodations SEC-031, FERPA export-metadata SEC-066), an admin of one tenant satisfies the gate for another tenant's `:user_id`/`:id`.
**Impact.** Account-admins are effectively cross-tenant for any standalone (non-course-scoped) resource whose service layer lacks its own accountID filter. The structural multiplier behind SEC-031 and SEC-066.
**Recommendation.** Make the admin branch tenant-aware (require resource/target accountID `== callerAccountID` unless super_admin); centralize the predicate.
**Verification:** 1-vote.

---

#### SEC-033 — GraphQL applies zero enrollment/role authorization: any in-tenant user reads any course's roster, assignments, modules
**Category:** Broken Object/Field-Level Authorization (BOLA/BFLA)
**Location:** `internal/graphql/resolver.go:119-131,202-251,428-480`
**Description.** The GraphQL package contains no enrollment/role/admin check. The only gate is tenant scope on the top-level `course(id)` lookup. `resolveCourse` accepts any in-tenant course id; nested `resolveCourseEnrollments`, `resolveCourseAssignments`, `resolveCourseModules` then return the full roster, assignments (including unpublished), and modules with no enrollment check. The REST twins are all behind `enrolled`/`instructor`.
**Impact.** A student can run `{ course(id: ANY) { enrollments { user_id role } assignments { name description } modules { id name } } }` for any course they are not enrolled in — FERPA roster exposure and embargoed-content leak, bypassing the REST guards.
**Recommendation.** Add a per-resolver authorization step mirroring `RequireEnrolled`; restrict `enrollments` to instructors/admins; filter unpublished assignments for non-staff.
**Verification:** 3-lens (conf 3 / ref 0). *(Independently confirmed by a second finder at `resolver.go:119`.)*

---

#### SEC-034 — GraphQL `user(id)` leaks every in-tenant user's email and login_id (selfOrAdmin bypass)
**Category:** Broken Object-Level Authorization / PII Exposure
**Location:** `internal/graphql/resolver.go:186-198,315-353`
**Description.** REST gates `GET /users/:id` with `selfOrAdmin`. `resolveUser` applies only tenant scope and no self-or-admin check; `buildUserMap` returns `email`, `login_id`, `sortable_name`, `short_name`, `locale`, `time_zone`.
**Impact.** Any authenticated user can iterate `{ user(id: N) { email login_id name } }` and harvest the PII of every user in the tenant (FERPA/COPPA concern; phishing fuel).
**Recommendation.** Allow only `id == caller userID` or admin; otherwise restrict to a public subset (name only).
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-035 — GraphQL nested assignments/modules resolvers are not tenant-scoped (cross-tenant content leak via `allCourses`)
**Category:** BOLA
**Location:** `internal/graphql/resolver.go:428-444,464-480`; repos `assignment.go:54-58`, `module.go:47-51`
**Description.** `resolveCourseAssignments`/`resolveCourseModules` pass no accountID; the repos filter only `course_id = ? AND workflow_state != 'deleted'`. Paired with SEC-001 (`allCourses` returns foreign course IDs), an attacker dumps cross-tenant assignment/module content.
**Impact.** Any authenticated user reads assignment names/descriptions/points and module structure for any course in any tenant.
**Recommendation.** Add accountID params scoped via `course_id IN (SELECT id FROM courses WHERE account_id = ?)`; enforce enrollment.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-036 — Account-admin can masquerade into a super_admin session, inheriting super_admin authority
**Category:** Privilege escalation via impersonation
**Location:** `internal/api/v1/handlers/users.go:604-644`; `internal/auth/jwt.go:60-77`
**Description.** `StartMasquerade` is gated by `RequireAdmin` and blocks self/nested masquerade and cross-tenant targets, but performs **no privilege-level check**. `GenerateMasqueradeToken` stamps the **target's** role into the JWT, and the session's authority is re-derived from `user_id` each request. An admin who can reach a super_admin row in their tenant (super_admins typically live in root `account_id=1`, reachable by that account's admins) obtains an effective super_admin session.
**Impact.** A `role=admin` user can elevate to a super_admin session by impersonating a co-resident super_admin, gaining cross-tenant platform-operator powers. Logged as `masquerade_start` (detectable) but not prevented.
**Recommendation.** Reject masquerade when `targetUser.Role` outranks the caller (admins may not impersonate admin/super_admin); add a role-precedence check before minting the token.
**Verification:** 1-vote.

---

#### Theme C — Authentication / session / SSO

---

#### SEC-037 — Session cookie minted WITHOUT the `Secure` flag on every federated/MFA/passkey login path
**Category:** Session management / cookie security
**Location:** `internal/auth/sso_handler.go:206,293`; `oidc.go:228`; `saml.go:737`; `handlers/mfa.go:301,368`; `handlers/passkeys.go:301,327-353`
**Description.** Only the local-password path (`users.go:57-70 setAuthCookie`) sets `Secure: cfg.Environment=="production"`. Every other path that mints the identical `paper_session` cookie hardcodes the `fiber.Cookie` struct and **omits `Secure`** (Go zero value = false): SAML ACS, OIDC callback, CAS, LDAP, TOTP step-up, recovery-code, passkey login. The OIDC state/nonce cookies and passkey ceremony cookies are likewise non-Secure. A grep for `Secure` across `internal/auth` and the passkey/oauth2 handlers returns zero hits.
**Impact.** For any tenant using SSO/MFA/passkey (the enterprise/K-12 norm), the 24h session JWT is sent over plaintext HTTP. A network-position attacker (rogue Wi-Fi, MITM, downgrade, mixed-content sub-resource) captures and replays the full session until JWT expiry. HSTS only partially mitigates (not first contact, non-browser clients, or non-HSTS subdomains).
**Recommendation.** Route every session-cookie write through one shared helper that sets `Secure: cfg.Environment=="production"` (or `X-Forwarded-Proto==https`); apply to OIDC state/nonce and passkey ceremony cookies. Add a test asserting no `paper_session` cookie literal is built outside the helper.
**Verification:** 3-lens (conf 3 / ref 0). *(Reported under two dimensions — authn-session and headers-cors-csrf — same root cause.)*

---

#### SEC-038 — SAML assertions with no `<Conditions>` element bypass AudienceRestriction and expiry validation
**Category:** Authentication / SAML assertion validation
**Location:** `internal/auth/saml.go:583-630`
**Description.** All `NotBefore`/`NotOnOrAfter`/`AudienceRestriction` validation is nested inside `if assertion.Conditions != nil`. A signed assertion that omits `<Conditions>` skips the F-054 audience check entirely and gets a 15-minute default cache TTL. The audience check is the documented defense against replaying a different SP's assertion behind a shared IdP cert — structurally unreachable when Conditions is absent.
**Impact.** Cross-SP assertion replay / authentication bypass: a signed-but-Conditions-less assertion is accepted without audience binding or freshness, letting an attacker authenticate as the assertion's subject in tenants sharing an IdP.
**Recommendation.** Fail closed when Conditions is absent: require a non-nil `<Conditions>` with `<AudienceRestriction>` matching the SP entity ID and a `NotOnOrAfter`. Move enforcement out of the `Conditions != nil` guard.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-039 — No `InResponseTo`/AuthnRequest-ID binding: ACS accepts unsolicited responses and forged login (SAML login CSRF)
**Category:** Authentication / SAML response binding
**Location:** `internal/auth/saml.go:447-455,287,328,509-578`
**Description.** `InitiateLogin` mints a random AuthnRequest ID but never persists it. `HandleACS` parses `InResponseTo` (and `SubjectConfirmationData.InResponseTo`) but never compares them against an outstanding request. There is no request-tracking store. The ACS endpoint cannot distinguish SP-initiated from arbitrary unsolicited responses.
**Impact.** Forced-login / login CSRF and unsolicited-response acceptance: an attacker who obtains a validly-signed, in-window assertion can POST it to `/auth/saml/acs` to authenticate as themselves or force a victim to log in as an attacker-controlled identity.
**Recommendation.** Persist each AuthnRequest ID (keyed to session/RelayState) with a short TTL; require `InResponseTo` to match an outstanding request, then consume it. Gate IdP-initiated SSO behind an explicit per-provider flag.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### Theme D — Crypto / secrets

---

#### SEC-040 — OneRoster connection `ClientSecret` stored plaintext despite "encrypted at rest" comment
**Category:** Cryptographic hygiene / plaintext secret column
**Location:** `internal/domain/models/oneroster_connection.go:11`
**Description.** The field carries `// encrypted at rest, never serialized`, but no code encrypts it. The handler copies raw input (`oneroster.go:121,189`), the service writes it unchanged, the JSON marshaller reads it raw, and it is used verbatim as the HTTP Basic credential (`oneroster_service.go:300`). No `secretbox.Encrypt`/`Decrypt` anywhere on this path.
**Impact.** Every tenant's OneRoster SIS OAuth2 client secret sits in cleartext in the DB, violating the project's "any DB-resident secret round-trips through secretbox" control. A DB read yields live SIS credentials granting access to the district's roster system (student PII). The misleading comment defeats reviewer scrutiny.
**Recommendation.** Encrypt via `auth.Encrypt` on Create/Update (ciphertext column, LDAP pattern); decrypt only in `fetchToken`. Add a backfill like `BackfillLDAPBindPasswords`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-041 — Stored XSS in portfolio HTML/PDF export — section `Content` injected unescaped
**Category:** Stored XSS
**Location:** `internal/service/portfolio_service.go:525,676`
**Description.** `PortfolioSection.Content` is never run through `SanitizeHTML` on write (`portfolio.go:401,689,802` assign verbatim; only Description is sanitized). On export, `generateStaticHTML`/`generatePrintHTML` inject `sec.Content` raw into HTML — Title is `escapeHTML`'d, Content is concatenated. `ExportAsStaticSite`/`ExportAsPDF` return downloadable HTML.
**Impact.** Stored XSS against any user (typically teacher/admin) who exports a student's portfolio to HTML/PDF, or any visitor to a hosted exported site. Full script execution (DOM access, CSRF-token theft, action-on-behalf). The in-app SPA renders Content as text and is safe, which is why this export path is easy to miss.
**Recommendation.** Run section/reflection/comment `Content` through `SanitizeHTML` at write time, and additionally sanitize at export rather than concatenating raw.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-042 — Stored XSS via `javascript:` href on the public (unauthenticated) portfolio page
**Category:** XSS
**Location:** `web/src/pages/PortfolioPublicPage.jsx:752,763,619`; backend `portfolio.go:213-214,295-299`
**Description.** `PortfolioPublicPage` renders `href={portfolio.linkedin_url}`, `href={portfolio.website_url}`, `href={artifact.url}` with no scheme validation; the backend stores these verbatim (only Description sanitized). React 18 does not block `javascript:` hrefs at runtime. The page is served unauthenticated (`/portfolios/public/:slug`, `router.go:818-819`).
**Impact.** A portfolio owner sets a URL field to `javascript:fetch('/api/v1/...')`. When a teacher/admin/parent/visitor clicks the link, script runs in the LMS app origin with the victim's session. Unauthenticated reachability widens the surface beyond the course.
**Recommendation.** Validate URL scheme server-side on create+update (allow only http/https/mailto/tel; reject javascript:/data:/vbscript:) and sanitize on the frontend before binding to href/src.
**Verification:** 3-lens (conf 2 / ref 1).

---

#### SEC-043 — Stored XSS: quiz `question_text` rendered without sanitization on instructor pages; QTI/IMSCC import bypasses server `SanitizeHTML`
**Category:** Stored XSS
**Location:** `web/src/pages/QuizStatisticsPage.jsx:215` (+ `ItemAnalysisPage.jsx:125`, `ItemBankManagerPage.jsx:344,407`, `StimulusEditorPage.jsx:202`); root cause `internal/service/imscc_parser.go:1114`, `qti_parser.go:281`
**Description.** Four privileged-user pages render `question_text` via `dangerouslySetInnerHTML` with no `sanitizeHTML` (only `String().slice()`). The REST create/update handlers sanitize, but the QTI/IMSCC import path persists `QuestionText` verbatim. An imported `.imscc`/QTI package can seed `<img onerror>`/`<svg onload>` that fires when an instructor opens statistics/item-analysis.
**Impact.** A teacher importing a malicious package gets stored XSS in the LMS origin with the victim instructor/admin's privileges. `paper_session` is httpOnly, but the `paper_csrf` cookie is JS-readable by design, enabling forged state-changing requests (grade/role changes, masquerade start) — privilege-escalation-grade XSS.
**Recommendation.** Wrap all four sinks in `sanitizeHTML()`; **and** call `service.SanitizeHTML` on `QuestionText` in the import path (the stronger fix). Sanitize answer/feedback fields too.
**Verification:** 3-lens (conf 3 / ref 0). *(A second adversarial pass confirmed `ItemBankManagerPage`/`ItemAnalysisPage`/`StimulusEditorPage` independently.)*

---

#### Theme E — SSRF

---

#### SEC-044 — Production SSRF-guarded HTTP clients follow redirects, bypassing the guard with a 302 to internal/metadata
**Category:** SSRF
**Location:** `internal/service/notification_delivery_service.go:573` (also `internal/auth/cas.go:31`, `oneroster_service.go:53`, `oidc.go:271`)
**Description.** `ValidateExternalURL` runs only on the original URL. Every production client uses a default `http.Client` with no `CheckRedirect`, so Go follows up to 10 redirects without re-validating. An attacker registers a guard-passing HTTPS host that returns `302 Location: http://169.254.169.254/...` (or RFC1918/custom port). The webhook path is worst: created by **any** authenticated user (`POST /users/self/communication_channels`, no admin guard). The two super-admin **test** endpoints already set `CheckRedirect: http.ErrUseLastResponse` with a "Wave 3 audit C1" comment documenting this exact bypass — the fix was never applied to production paths.
**Impact.** SSRF to cloud metadata (credential theft), internal admin panels, K8s API, arbitrary internal ports — a low-privilege-to-internal-network primitive. CAS/OneRoster/OIDC variants leak Basic-auth/bearer creds to the redirected host.
**Recommendation.** Set `CheckRedirect` to re-run `ValidateExternalURL` per hop (or `ErrUseLastResponse`) on the production clients; inject a redirect-limited client into the OIDC library via `oidc.ClientContext`/`oauth2.HTTPClient`.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### Theme F — DoS / resource exhaustion

---

#### SEC-045 — IMSCC/QTI content import has no zip decompression-bomb guard (in-memory `ReadAll` of every entry)
**Category:** DoS / decompression bomb
**Location:** `internal/qti/imscc.go:50-89`; `internal/service/imscc_parser.go:343-446,1423-1496`
**Description.** Both Common Cartridge paths open the zip and `io.ReadAll` every entry into a map with no cap on decompressed size, per-entry size, or entry count. `extractFiles` trusts `f.UncompressedSize64`. The upload routes bound only the **compressed** upload (default 5120 MB). DEFLATE ratios can exceed 1000:1.
**Impact.** Any instructor can upload a small zip bomb that decompresses to gigabytes in memory, OOM-killing the pod. `ExpensiveOpRateLimit` (5/min) does not help — a single request suffices.
**Recommendation.** Wrap each `f.Open()` in `io.LimitReader`; track a running aggregate total and abort past a cap (200-500 MB); cap `len(zr.File)`; reject implausible `UncompressedSize64`. Stream large assets to disk.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-046 — GraphQL pagination `perPage` has no upper cap (unbounded SQL LIMIT / memory)
**Category:** DoS / unbounded query
**Location:** `internal/graphql/resolver.go:133-156,428-480,520-540`
**Description.** `getIntArgOr` applies no maximum; resolvers pass `perPage` straight into `PaginationParams`, used verbatim as SQL `LIMIT`. GraphQL bypasses the REST pagination middleware (`MaxPerPage=100`). `{ allCourses(perPage: 100000000){ id } }` issues a single huge query.
**Impact.** Any authenticated user can force giant result sets, exhausting DB/app memory; amplified by missing rate limit (SEC-049) and nested expansion.
**Recommendation.** Clamp `perPage` (≤100) and `page` (≥1) in `getIntArgOr`/each resolver; better, centralize in `repository.PaginationParams`.
**Verification:** 3-lens (conf 3 / ref 0). *(See also SEC-050: negative `perPage` → GORM `Limit(-1)` = unlimited.)*

---

#### Theme G — Additional High-severity items

---

#### SEC-047 — Module item Get/Update/Delete/Move lack parent-tie and tenant scope
*Downgraded to Medium by validator — see SEC-061.*

#### SEC-048 — Unauthenticated-context write: `CalendarEvent.CreateEvent` trusts attacker-supplied `context_type`/`context_id`
**Category:** Broken Access Control / missing object-level authz
**Location:** `internal/api/v1/handlers/calendar_events.go:89-128`; `internal/service/calendar_service.go:23`
**Description.** `POST /calendar_events` is mounted with only `Protected()`+CSRF. The handler binds `ContextType`/`ContextID` from the body and persists; the service validates only Title/StartAt and never checks the caller may write to that context. Update/Delete are correctly gated; Create is not. No `callerAccountID` scoping.
**Impact.** Any authenticated user can inject a calendar event into any Course context (visible to all enrolled members) or any user's personal calendar, including other tenants. Cross-course/cross-tenant calendar spam and content injection.
**Recommendation.** Branch on `ContextType`: Course→`RequireCourseEnrolled`/Instructor; User→force `ContextID = caller`; reject unknown types (default-deny); scope to `callerAccountID(c)`.
**Verification:** 3-lens (conf 3 / ref 0). *(Independently confirmed by an IDOR finder at `calendar_events.go:89`.)*

---

#### SEC-049 — `/graphql` endpoint has no rate limit
*See Section: Medium — consolidated with SEC-046/SEC-050 amplification. Retained as Medium (SEC-073).*

#### SEC-050 — Discussion checkpoints: missing parent-tie + classmate progress IDOR
**Category:** IDOR / missing parent-tie
**Location:** `internal/api/v1/handlers/discussion_checkpoints.go:154`
**Description.** All checkpoint endpoints take `:topic_id`/checkpoint `:id` from the URL, never verify they belong to `:course_id`, and use `ListByTopicID(ctx, topicID, 0)`/`UpdateCheckpoint`/`DeleteCheckpoint(id)` with `accountID=0`. `GetUserProgress` (`enrolled`) reads `?user_id=N` with no self/staff gate; `Update`/`DeleteCheckpoint` (`instructor`) verify the caller's course but not the checkpoint's course.
**Impact.** (1) Any enrolled student reads any classmate's discussion progress (FERPA). (2) Any instructor can update/delete any checkpoint in any course of any tenant by ID.
**Recommendation.** Assert `topic.CourseID == :course_id` + tenant match; for `GetUserProgress` require `caller == user_id` OR staff. Thread accountID through the repo.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-051 — Course paces: missing parent-tie; `DeleteCoursePace` has no tenant scope
**Category:** IDOR / missing parent-tie
**Location:** `internal/api/v1/handlers/course_paces.go:176`; `course_pace_service.go:53`
**Description.** Course-pacing routes are `instructor`-gated on `:course_id` but Get/Update/Delete never verify `pace.CourseID == :course_id`. `DeleteCoursePace` calls `Delete(ctx, paceID)` with no accountID and no course tie.
**Impact.** An instructor of any course in tenant A can delete a course pace belonging to any course in any tenant by guessing the sequential ID; same-tenant cross-course read/modify of pacing data.
**Recommendation.** Fetch via `GetByID(id, callerAccountID)`, assert `pace.CourseID == :course_id`, and give `Delete` an accountID parameter.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-052 — `ListSubmissionComments`/`CreateSubmissionComment` let any enrolled student read/post on another student's submission
**Category:** IDOR / Broken Object-Level Authorization
**Location:** `internal/api/v1/handlers/submissions.go:359-461`
**Description.** `GET/POST .../submissions/:user_id/comments` (`enrolled`) resolve the target submission via `GetByAssignmentAndUser(assignmentID, :user_id, callerAccountID)` (tenant scope only), then list/create comments with no owner/instructor/observer check.
**Impact.** A student can read private teacher-to-student feedback on any classmate's submission and inject comments onto another student's submission (impersonation/harassment). Comments often carry grading rationale and PII.
**Recommendation.** Apply the same owner/instructor/observer guard as the quiz path before resolving or mutating comments.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-053 — Cross-tenant IDOR: any account-admin can read/tamper with another tenant's Data Processing Agreements
**Category:** Multi-tenant isolation / business logic
**Location:** `internal/api/v1/handlers/coppa.go:197-249`; `coppa_service.go:182-198`; `parental_consent.go:107-117`
**Description.** The DPA update/read path loads purely by PK with no account scoping. `UpdateDPA` → `GetDPA(ctx, dpaID)` → `dpaRepo.FindByID(ctx, id)` runs `First(&agreement, id)` with no `account_id`. Route is `admin` (tenant-scoped account admin, not super-admin). `ListDPAs` scopes by account; the by-ID path does not.
**Impact.** A tenant admin can disclose and silently modify another district's DPAs — flip `status` to active/expired, rewrite retention period/vendor, read confidential contract data. Cross-tenant integrity + confidentiality breach in a K-12 privacy subsystem.
**Recommendation.** Thread `account_id` into `GetDPA`/`UpdateDPA` and the repo (`WHERE id = ? AND account_id = ?`); 404 on miss; super-admin bypass.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-054 — Cross-tenant IDOR: any account-admin can revoke another tenant's parental (COPPA) consent
**Category:** Multi-tenant isolation / business logic
**Location:** `internal/api/v1/handlers/coppa.go:120-133`; `coppa_service.go:121-136`; `parental_consent.go:34-40`
**Description.** `RevokeConsent` resolves by PK (`FindByID(ctx, consentID)`, no scoping), sets `Status="revoked"`, updates. The signature takes a `revokerID` but never uses it for authorization. Route is tenant-admin only.
**Impact.** An admin of account A can revoke COPPA parental consent for a child in account B by iterating consent IDs — a direct COPPA-compliance integrity violation that can block the child's data processing/account use.
**Recommendation.** Scope the lookup by the subject student's `account_id` vs caller's (404 on mismatch; super-admin bypass); use the unused `revokerID`/account for the check.
**Verification:** 3-lens (conf 3 / ref 0).

---

### MEDIUM

---

#### SEC-055 — `required_admin` MFA policy does not force super_admin accounts to enroll or step up
**Category:** MFA policy enforcement
**Location:** `internal/auth/login_pipeline.go:347-358`
**Description.** `decideMFAGate` special-cases only `user.Role == "admin"`. A `super_admin` (treated as admin by the auth middleware) falls through to the non-admin branch and gets `mfaGateNone` if not enrolled — the highest-privilege role is exempt from the policy meant to mandate admin MFA.
**Impact.** A tenant configuring `mfa_policy=required_admin` leaves its super_admins able to log in with a password alone.
**Recommendation.** `if user.Role == "admin" || user.Role == "super_admin"`; introduce a shared `isAdminRole` helper.
**Verification:** 1-vote.

---

#### SEC-056 — OAuth2 developer-key client secret stored plaintext and compared with timing-unsafe `!=`
**Category:** Cryptographic hygiene / plaintext secret + timing leak
**Location:** `internal/service/oauth2_service.go:128`; `internal/api/v1/handlers/oauth2.go:307`
**Description.** The client secret is persisted as `text NOT NULL` (no encrypt/hash) and verified at token exchange with `devKey.ClientSecret != clientSecret` (non-constant-time, short-circuits on first differing byte). Contrast OIDC/LDAP which use `auth.Encrypt`.
**Impact.** (1) DB read yields every tenant's OAuth client secrets in cleartext → mint API tokens for arbitrary users. (2) Byte-by-byte timing recovery against the public `/login/oauth2/token` endpoint.
**Recommendation.** Store a SHA-256/bcrypt hash (or `secretbox.Encrypt`); compare via `subtle.ConstantTimeCompare`. Migrate existing rows.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-057 — `JWT_SECRET` length is never enforced — a 1-character production secret passes validation
**Category:** Cryptographic hygiene / weak key strength
**Location:** `internal/config/config.go:69-80`
**Description.** `Validate` only rejects the exact hardcoded default literal; any other value (including 1 byte) is accepted. The fatal message claims "at least 32 characters" but no length/entropy check exists. `JWT_SECRET` is the HS256 key for all session, masquerade, and pending tokens.
**Impact.** A short/guessable secret makes the HS256 signature brute-forceable offline from any captured token → full session/identity forgery (incl. admin/masquerade) across all tenants.
**Recommendation.** In `Validate()`, for `ENVIRONMENT=production`, `log.Fatal` when `len(JWTSecret) < 32` (or require base64-of-32-bytes / measure entropy).
**Verification:** 1-vote.

---

#### SEC-058 — Student-submitted assignment `Body` persisted without server-side `SanitizeHTML`
**Category:** Stored XSS / sanitization control gap
**Location:** `internal/api/v1/handlers/submissions.go:222`; `submission_service.go:140,331`
**Description.** `CreateSubmission` and the online_text_entry path store the student rich-text `Body` verbatim. No `SanitizeHTML` on handler or service, violating the "sanitize every rich-content field on create+update" control. Rendered via `dangerouslySetInnerHTML` on multiple pages.
**Impact.** Student-stored HTML; the only remaining defense is frontend DOMPurify. The Canvas-compatible API returns raw HTML to third-party clients; the permissive client allowlist (SEC-064) still permits iframe/object/embed/style/data: for phishing/redress against a grading teacher.
**Recommendation.** Wrap `Body` in `service.SanitizeHTML` on create and the group-update copy path.
**Verification:** 1-vote.

---

#### SEC-059 — Legacy question-bank `QuestionText` accepted and stored without `SanitizeHTML`
**Category:** Stored XSS / sanitization control gap
**Location:** `internal/api/v1/handlers/question_banks.go:127`
**Description.** `AddQuestion`/`UpdateQuestion` BodyParse a full `QuestionBankEntry` (incl. `QuestionText`) and pass it straight to the service with no sanitize, unlike the modern quiz_questions/quiz_item_banks paths. Stored value rendered raw on several instructor pages (incl. `QuizStatisticsPage.jsx:215`).
**Impact.** An instructor (or co-instructor) stores markup rendered via `dangerouslySetInnerHTML` with no sanitize — stored XSS reachable by another instructor/TA viewing analytics.
**Recommendation.** Sanitize `entry.QuestionText` (and rendered Answers) in `AddQuestion`/`UpdateQuestion`; add `sanitizeHTML()` to the raw sinks.
**Verification:** 1-vote.

---

#### SEC-060 — Frontend `dangerouslySetInnerHTML` sinks render `question_text` with no DOMPurify
**Category:** XSS sink (missing client sanitization)
**Location:** `web/src/pages/QuizStatisticsPage.jsx:215` (+ `ItemAnalysisPage.jsx:125`, `ItemBankManagerPage.jsx:344,407`, `StimulusEditorPage.jsx:202`)
**Description.** Four instructor pages pass `question_text` directly to `dangerouslySetInnerHTML` with no `sanitizeHTML()`. `.slice()` truncates but does not sanitize. Server sanitizes interactive writes but not QTI/IMSCC import or the legacy bank path (SEC-043/SEC-059/SEC-062).
**Impact.** Client-side endpoint of the stored-content gaps; injected `<img onerror>`/`<svg onload>` from imported/legacy questions executes in the instructor's browser.
**Recommendation.** Route every `question_text` render through `RichContentViewer`/`sanitizeHTML`, as `QuizReviewPage.jsx:149`/`ItemPlayer.jsx:164` already do.
**Verification:** 1-vote.

---

#### SEC-061 — Module item Get/Update/Delete/Move lack parent-tie and tenant scope
**Category:** Cross-tenant content tampering / IDOR
**Location:** `internal/api/v1/handlers/module_items.go:43`; `module_service.go:72`
**Description.** Routes `/courses/:course_id/modules/:module_id/items/:item_id` resolve the ContentTag via `GetItem(ctx, id)` → `moduleItemRepo.FindByID(ctx, id)` with no accountID and no verification the item's module belongs to `:course_id`. `MoveItemToModule` validates nothing.
**Impact.** A teacher/TA can read, retitle, unpublish, delete, or move any module item in any other course/tenant by item_id (course-content integrity / cross-tenant tampering).
**Recommendation.** Thread `callerAccountID(c)` and verify `ContextModule → course → account_id` matches the caller's tenant and the URL `:course_id`/`:module_id`.
**Verification:** 1-vote.

---

#### SEC-062 — QTI/IMSCC/blueprint import paths copy `QuestionText`/`Body` verbatim without `SanitizeHTML`
**Category:** Stored XSS via import
**Location:** `internal/service/qti_import_service.go:180,233,317`; `qti_parser.go:281`; `imscc_parser.go:1671,1967`; `batch_service.go:302`; `blueprint_sync.go:350`
**Description.** Content-import and copy services build quiz questions/items by copying rich fields straight from parsed package data with no sanitize, bypassing the control at this trust boundary.
**Impact.** An instructor importing a crafted package (or a blueprint sync from a compromised source course) introduces unsanitized HTML that renders raw in instructor analytics sinks (SEC-060).
**Recommendation.** Apply `service.SanitizeHTML` to QuestionText/Body/stimulus content inside the import and blueprint-copy services.
**Verification:** 1-vote.

---

#### SEC-063 — Conversation reply `Body` stored without `SanitizeHTML` (inconsistent with bulk-send)
*Reclassified as Low — see SEC-090.*

#### SEC-064 — Frontend DOMPurify allowlist permits iframe/object/embed/form/style/data: for all rich content
*Reclassified as Low — see SEC-091.*

#### SEC-065 — OIDC discovery fetches token/authorization/JWKS sub-endpoints without re-validating against the SSRF guard
**Category:** SSRF
**Location:** `internal/auth/oidc.go:266`
**Description.** `ValidateExternalURL` runs on `OIDCIssuerURL`, then `oidc.NewProvider` fetches `.well-known/openid-configuration` and trusts the `authorization_endpoint`/`token_endpoint`/`jwks_uri` from the response body without re-validating. `cfg.Exchange` POSTs the code + client_secret to the discovered token endpoint.
**Impact.** A tenant admin who controls the discovery JSON (issuer passed the guard) can point `token_endpoint` at `169.254.169.254` or an internal host → SSRF with exfiltration of the OIDC `client_secret`.
**Recommendation.** After `oidc.NewProvider`, run `ValidateExternalURL` on `AuthURL`/`TokenURL`/JWKS before any exchange; reject endpoints leaving public-unicast or changing scheme/port.
**Verification:** 1-vote.

---

#### SEC-066 — `GET /sections/:id` leaks cross-tenant sections to admins (untenanted `FindByID` + isAdmin bypass)
**Category:** Cross-tenant IDOR (admin-scoped)
**Location:** `internal/api/v1/handlers/sections.go:89-108`; `authz.go:26-72`
**Description.** Route has no course/tenant middleware. Handler does `sectionRepo.FindByID(c.Context(), uint(id))` (no accountID) then `RequireCourseEnrolled(c, section.CourseID)`. For non-admins this is tenant-safe; the `isAdmin(c)` helper returns `nil` after only checking `role == "admin"`, never verifying the section belongs to the admin's tenant.
**Impact.** A non-root account admin in tenant A can read any section (id, name, sis_section_id, course_id, dates) of any course in any other tenant.
**Recommendation.** Assert the section's owning course belongs to `callerAccountID(c)`; have `sectionRepo.FindByID` take accountID joining through `courses.account_id`.
**Verification:** 1-vote.

---

#### SEC-067 — `isAdmin` role check bypasses course/tenant scope in course-role middleware and ResourceAuthorizer
**Category:** Broken Access Control (systemic)
**Location:** `internal/api/v1/middleware/permissions.go:133-137,172-175,250-268`; `authz.go:26-72`
**Description.** `RequireCourseRole`/`RequireEnrolled`/`RequireInstructor` and `RequireCourseEnrolled`/`RequireCourseInstructor` treat `role==admin` as an unconditional pass for the `:course_id` gate with no check that the course is in the admin's tenant — unlike `RequireAdmin`. Safe only when the downstream handler independently scopes by `callerAccountID`; already broken on `GET /sections/:id` (SEC-066).
**Impact.** Any admin passes course-scoped middleware for courses in other tenants; the only barrier is per-handler re-scoping, an unenforced invariant.
**Recommendation.** When `isAdmin` is true, verify the resolved course's `account_id == callerAccountID(c)` (or super_admin). Centralize.
**Verification:** 1-vote.

---

#### SEC-068 — `ListFolderFiles` exposes file metadata of any folder with no authorization or tenant scope
*Consolidated with SEC-017 (same route/sink, reported by two dimensions). Severity High per SEC-017; this dimension rated it Medium.* Verification: 1-vote.

#### SEC-069 — `DeleteAnnotation` authorizes against caller-supplied `course_id` instead of the annotation's real course
**Category:** Broken Access Control / IDOR
**Location:** `internal/api/v1/handlers/document_annotations.go:267`; `document_annotation_service.go:143`
**Description.** `DELETE /annotations/:id` (no route guard) takes `course_id` from the URL/query and passes that attacker-controlled value to the service, which does `annotationRepo.FindByID(ctx, annotationID, 0)` and, when caller != owner, checks `isInstructor(userID, courseID)` against the **supplied** courseID. It never verifies the annotation belongs to that course/tenant.
**Impact.** A teacher/TA in any course can delete an annotation belonging to a different course/tenant by passing their own `course_id` — destroying grading feedback they have no authority over.
**Recommendation.** Resolve the course from the annotation (`getCourseIDForAnnotation`, which tenant-scopes via `callerAccountID`); run `RequireCourseInstructor` on that; ignore caller-supplied `course_id`; stop passing `accountID=0`.
**Verification:** 1-vote.

---

#### SEC-070 — Document annotations on a submission readable by any enrolled student (`ListAnnotations`/`GetAnnotationSummary`)
**Category:** IDOR / Broken Object-Level Authorization
**Location:** `internal/api/v1/handlers/document_annotations.go:98-130,396-424`
**Description.** Routes (`enrolled`) resolve the submission via `GetByAssignmentAndUser(... callerAccountID)` (tenant scope only) and return annotations/summary with no owner/instructor/observer check. `CreateAnnotation` (POST, `enrolled`) lets the caller annotate another student's submission.
**Impact.** Inline grader annotations (private teacher markup) on any classmate's submission are exposed to any enrolled student; POST lets a student write annotations onto another's submission.
**Recommendation.** Add the owner/instructor/observer check on the resolved submission before list/summarize/create.
**Verification:** 3-lens (conf 3 / ref 0).

---

#### SEC-071 — Outcome results endpoint lets any enrolled student read a classmate's mastery results (`?user_id=`)
**Category:** Authorization / FERPA peer leak
**Location:** `internal/api/v1/handlers/learning_outcomes.go:398`
**Description.** `GET /courses/:course_id/outcome_results` (`enrolled`) calls `ListResultsByUserAndContext(ctx, user_id, "Course", course_id)` when `?user_id` is supplied, with no check that the caller is the target student or staff. The handler's own audit code acknowledges "reading another student's outcome mastery results."
**Impact.** Any enrolled student can read any classmate's outcome mastery scores via `?user_id=<classmate>`. In-course peer FERPA leak.
**Recommendation.** When `user_id != callerID`, require instructor/admin; otherwise restrict to the caller's own id.
**Verification:** 1-vote.

---

#### SEC-072 — Discussion entry V2 version-history read has no parent-tie or tenant scope
**Category:** Cross-tenant content/edit-history leak
**Location:** `internal/api/v1/handlers/discussions_v2.go:162`; `discussion_v2_service.go:311`
**Description.** `GET .../entries/:entry_id/versions` (`enrolled`) calls `GetEntryVersions(ctx, entryID)` → `versionRepo.ListByEntryID(ctx, entryID)` with no tenant scope and no `:topic_id`/`:course_id` tie.
**Impact.** Any user enrolled in any course can read the full edit history (all prior versions + authoring user_id) of any discussion entry in any tenant.
**Recommendation.** Resolve scoped to `callerAccountID`, assert the topic belongs to `:course_id`, then return versions (404 on cross-tenant).
**Verification:** 1-vote.

---

#### SEC-073 — `/graphql` endpoint has no rate limit; no depth/complexity/field-repetition budget
**Category:** DoS / rate-limit & complexity gap
**Location:** `internal/api/v1/router.go:627-628`; `internal/graphql/schema.go:79-103`; `handlers/graphql.go:20-61`
**Description.** `/graphql` is mounted with no `RateLimitMiddleware`/`ExpensiveOpRateLimit` and no role restriction. The recursive parser has no depth cap; resolvers impose no complexity/field-count limit; the global Fiber BodyLimit is large. Each list field issues its own paginated query (N+1 fan-out across `allCourses`). Root-field repetition is executed N times with no dedup.
**Impact.** A single authenticated request can force thousands of large DB scans, or deep nesting/repetition to drive CPU/memory; sustained resource exhaustion with no per-route throttle.
**Recommendation.** Enforce max selection-set depth and a total field/complexity budget; cap `perPage`; dedup/merge duplicate top-level fields; add `ExpensiveOpRateLimit()` (or a dedicated limiter) to `/graphql`; reduce the effective body limit for GraphQL.
**Verification:** 1-vote (consolidates the DoS-dimension and graphql-dimension reports).

---

#### SEC-074 — Negative `perPage` in GraphQL becomes GORM `Limit(-1)` (no limit) — full-table dump
**Category:** Resource exhaustion / DoS
**Location:** `internal/graphql/resolver.go:133-156,520-541`; `course.go:61-62`
**Description.** `getIntArgOr` applies no lower clamp; GORM treats `Limit(-1)` as "no limit". `allCourses(perPage: -1)` selects the entire `courses` table (and, via SEC-001, across all tenants).
**Impact.** A single query forces an unbounded full-table scan/serialization — a cheap DoS, compounded with the all-tenant leak.
**Recommendation.** Clamp `perPage` (`1 ≤ perPage ≤ MaxPerPage`) and `page ≥ 1`; centralize in `PaginationParams`.
**Verification:** 1-vote.

---

#### SEC-075 — GraphQL `user(id)`/`course(id)` bypass REST object-level authz (selfOrAdmin/enrolled)
*Consolidated with SEC-033/SEC-034 — same resolvers, within-tenant scope. Verification: 1-vote (this dimension) reinforcing the 3-lens findings.*

#### SEC-076 — GraphQL exposes unpublished assignments (no workflow_state / enrolled gate)
**Category:** Broken Object-Level Authorization
**Location:** `internal/graphql/resolver.go:159-171,428-444`; `assignment.go:54-58`
**Description.** `assignment(id)` and `course.assignments` apply only tenant scope; `ListByCourseID` filters only `workflow_state != 'deleted'`, returning `unpublished`. No enrollment/visibility filter.
**Impact.** A student can read draft/unpublished assignment content (name, description, due/unlock/lock dates, points) for any course in the tenant, including courses they are not enrolled in.
**Recommendation.** Gate on enrollment; filter `workflow_state` for non-privileged callers.
**Verification:** 1-vote.

---

#### SEC-077 — OAuth2 token endpoint is not rate-limited (client_secret brute-force / flood)
**Category:** DoS / rate-limit coverage gap
**Location:** `internal/api/v1/routes_public.go:38`; `handlers/oauth2.go:191-242`
**Description.** `api.Post("/login/oauth2/token", ...)` is registered without `authLimit`, unlike every other public credential endpoint. The handler validates `client_secret` for authorization_code/refresh_token grants.
**Impact.** Unauthenticated attacker can flood the endpoint or brute-force client_secret/refresh_token values with no per-IP cap (compounding the timing leak in SEC-056).
**Recommendation.** Add `authLimit` (`AuthRateLimit`) to the route.
**Verification:** 1-vote.

---

#### SEC-078 — Bulk enrollment/message/date-shift endpoints accept unbounded input arrays
**Category:** DoS / unbounded batch
**Location:** `internal/api/v1/handlers/batch.go:145-260`
**Description.** `BulkEnrollUsers`/`BulkUpdateAssignmentDates` reject only empty arrays; no max length. A 10 MB JSON body can carry tens of thousands of entries, each driving a DB write. Routes carry `ExpensiveOpRateLimit` (5/min) + instructor/admin auth, bounding frequency but not per-request work.
**Impact.** An instructor can trigger tens of thousands of synchronous DB operations per request, repeatable 5×/min.
**Recommendation.** Enforce a max items per bulk request (e.g. 500-1000), 400 past the cap.
**Verification:** 1-vote.

---

#### SEC-079 — `CreateEnrollment` does not verify the target user belongs to the caller's tenant
**Category:** Multi-tenant isolation / business logic
**Location:** `internal/api/v1/handlers/enrollments.go:70-108`; `enrollment_service.go:55-76`
**Description.** `CreateEnrollment` takes `UserID`/`Type` from the body. `RequireInstructor` proves only that the caller teaches the URL course in their tenant. `Create` validates the Type and checks for duplicates (scoped to accountID) but never confirms the target user's `account_id` matches the course/caller tenant.
**Impact.** An instructor can enroll a foreign-tenant user (including as Teacher/TA) into their own course, creating cross-tenant linkage.
**Recommendation.** Load the target user under the course's `account_id` (`FindByID(ctx, userID, accountID)`) and reject if it doesn't resolve.
**Verification:** 1-vote.

---

#### SEC-080 — `BulkGrade` does not tie each entry's assignment to the URL course
**Category:** Broken Access Control
**Location:** `internal/api/v1/handlers/submissions.go:463-529`
**Description.** `POST /courses/:course_id/submissions/bulk_grade` (`instructor`) loops over `input.GradeData` and calls `Grade(entry.AssignmentID,...)` using only the caller's tenant. The `_ = courseID // validated above` comment is misleading: only the parse was validated, not that each `entry.AssignmentID` belongs to `:course_id`. The service gates by tenant (F-016) but not by course.
**Impact.** An instructor of course A can write bulk grades for assignments in sibling course B (same tenant) where they hold no role.
**Recommendation.** Verify each `assignment.CourseID == :course_id` (or caller is instructor of the assignment's actual course) before grading.
**Verification:** 1-vote.

---

#### SEC-081 — Currency-award rules have no per-milestone idempotency; re-grading re-awards currency
**Category:** Business-logic / economy integrity
**Location:** `internal/service/gamification/wiring/submission.go:55-129`; `effects/award_currency.go:57-89`; `gamification_wallet.go:121-154`
**Description.** The wallet idempotency key `(triggering_event_id, triggering_rule_id)` only dedupes retries of the same event. Each `Grade()` fires `OnGraded`, constructing a fresh `GamificationEvent` with a new ID. Achievement predicates are stateless. The only brake is the optional per-rule `cooldown_seconds`/`max_per_window`. The badge effect has `ON CONFLICT DO NOTHING`; the currency effect has no first-achievement guard.
**Impact.** A teacher re-grading (or a student resubmitting then re-graded) inflates a learner's currency/XP once per re-grade for any rule lacking a lifetime cap. Corrupts leaderboards and the points economy.
**Recommendation.** Add a milestone-level idempotency guard for `AwardCurrency` (dedupe on `(user_id, rule_id, object_type, object_id)` or a stable natural key), or make `max_per_window:{lifetime,1}` the default and validate at rule-create time.
**Verification:** 1-vote.

---

#### SEC-082 — Gamification currency can be farmed by repeating student-controlled actions (page view, discussion post)
**Category:** Business logic abuse / economy integrity
**Location:** `internal/service/content_view_service.go:49-60`; `wiring/content_view.go:39-138`; `wiring/discussion.go:40-50`; `award_currency.go:58-89`
**Description.** `RecordView` fires `OnViewed` on **every** page GET (not just first view); `discussion.go` fires `verb=posted` on every entry create. Each builds a fresh event/id, so the wallet idempotency index never collides. The only brake is the optional cooldown/cap.
**Impact.** A student can re-GET a page (or repeatedly post) to re-trigger any `verb=viewed`/`verb=posted` `AwardCurrency` rule lacking a cooldown, inflating spendable currency/leaderboard standing without limit — student-controlled, distinct from the instructor-triggered SEC-081.
**Recommendation.** Dedup view/post awards at the source (first-view only, or stable natural key), or make a cooldown/cap mandatory for student-self-triggerable verbs.
**Verification:** 1-vote.

---

#### SEC-083 — FERPA deletion walk (`EraseDependents`) omits student-authored free-text PII
**Category:** FERPA / data retention
**Location:** `internal/service/user_deletion_service.go:84-154`; `models/quiz_submission_answer.go:9`
**Description.** `EraseDependents` enumerates six tables and does **not** touch `quiz_submission_answers.answer` (a `type:text` essay/short-answer column), `document_annotations`, or portfolio artifact bodies. The table list is hardcoded, not derived from a PII registry.
**Impact.** After an approved FERPA deletion, the deleted user's essay answers and annotation content remain in plaintext, defeating the deletion guarantee. (Currently latent — `ProcessDeletion` has no wired HTTP caller yet.)
**Recommendation.** Add `quiz_submission_answers` (answer→NULL), `document_annotations`, and portfolio free-text to the walk; drive the column set from a single PII-column registry.
**Verification:** 1-vote.

---

#### SEC-084 — Cross-tenant calendar event injection via `POST /calendar_events`
*Consolidated with SEC-048 (write-side authz gap), confirmed independently by the IDOR finder. 3-lens (conf 3 / ref 0).*

#### SEC-085 — Cross-tenant conversation/inbox injection: `CreateConversation` accepts arbitrary recipient user IDs
**Category:** IDOR / Broken Object-Level Authorization (write)
**Location:** `internal/api/v1/handlers/conversations.go:201`; `conversation_service.go:32`
**Description.** `CreateConversation` takes `recipients []uint` from the body. The only authorization is the COPPA gate, which fires only for `coppa_strict`/`k5`/`m68` tenants. For all other tenants there is no validation that recipients belong to the caller's tenant or share a course; the service creates a participant row per recipient blindly.
**Impact.** In all non-COPPA tenants, any authenticated user can inject messages/spam/phishing into any user's inbox by guessing IDs, and enumerate recipients across tenants.
**Recommendation.** Validate every recipient resolves to a user in the caller's `account_id` (and shares a course per policy) before creating participants — universally, not only behind the COPPA gate.
**Verification:** 3-lens (conf 2 / ref 0).

---

#### SEC-086 — `ListRoles` trusts URL `:account_id` without a tenant tie
**Category:** Cross-tenant IDOR
**Location:** `internal/api/v1/handlers/custom_roles.go:53-75`
**Description.** `GET /accounts/:account_id/roles` passes the path `:account_id` directly to `ListRoles` with no `assertSameTenant` / `callerAccountID` substitution. `CreateRole` and DELETE carry the tie; the List read does not.
**Impact.** A tenant-A admin can read another tenant's custom role catalog (names, base role types, permission strings).
**Recommendation.** Use `callerAccountID(c)` or call `assertSameTenant` before listing.
**Verification:** 1-vote.

---

#### SEC-087 — `GetFile` lacks course parent-tie: any enrolled user reads sibling-course file metadata in the tenant
**Category:** Broken Access Control (IDOR)
**Location:** `internal/api/v1/handlers/files.go:157-173`
**Description.** `GET /courses/:course_id/files/:id` (`enrolled`) loads the attachment with `GetAttachment(id, callerAccountID)` (tenant-scoped) but, unlike `DeleteFile`, performs **no** parent-tie check (`ContextType=="Course" && ContextID==uint(courseID)`).
**Impact.** Within-tenant horizontal IDOR on file metadata (display_name, filename, content_type, size, md5, user_id) across all courses in the district.
**Recommendation.** Add the parent-tie check used in `DeleteFile`.
**Verification:** 1-vote.

---

#### SEC-088 — Soft-deleted files remain fully downloadable by ID (delete does not revoke access or purge bytes)
**Category:** Access control / data retention
**Location:** `internal/repository/postgres/attachment.go:23-45` (`FindByID`); `handlers/files.go:175-203,211`
**Description.** Soft-delete sets `workflow_state="deleted"` (not gorm `DeletedAt`). `FindByID` applies only the tenant filter, no `workflow_state != 'deleted'`. `DownloadFile`/`GetFile` go through `GetAttachment`→`FindByID` and never inspect `WorkflowState`. `DeleteFile` never calls `storageBackend.Delete`, so bytes persist.
**Impact.** After a "delete," any user who knows/enumerates the attachment ID and passes context authz can still fetch metadata and bytes; the object is never purged. Breaks the FERPA delete-revokes-access expectation.
**Recommendation.** Add `workflow_state != 'deleted'` to `FindByID` (or an explicit guard in download), and call `storageBackend.Delete` on delete (or a reconcile job).
**Verification:** 1-vote (confirmed twice across file-handling passes).

---

#### SEC-089 — QTI/IMSCC import upload route lacks `EnforceUploadSize` and rate limiter (decompression-bomb DoS)
**Category:** Resource exhaustion / DoS (file upload)
**Location:** `internal/api/v1/routes_quiz_extensions.go:50`; `internal/qti/imscc.go:68`
**Description.** `POST /courses/:course_id/qti_import` is mounted with only `instructor` — no `EnforceUploadSize`, no `ExpensiveOpRateLimit`, unlike sibling `content_imports`/`content_migrations`. The handler hands the file to `ImportIMSCC`→`openIMSCC`, which `io.ReadAll`s every zip entry into memory with no cap (SEC-045).
**Impact.** Any instructor can repeatedly POST small zip bombs; each expands unbounded in memory and the route is blocking. A few concurrent requests OOM-kill the pod — a less-protected entry point than `content_migrations`.
**Recommendation.** Add `EnforceUploadSize` + `ExpensiveOpRateLimit` to the route and per-entry/aggregate decompression caps in `openIMSCC`.
**Verification:** 1-vote.

---

#### SEC-090 — Calendar event `GetEvent`: any authenticated user reads any other user's private calendar event by ID
**Category:** IDOR / Broken Object-Level Authorization
**Location:** `internal/api/v1/handlers/calendar_events.go:75-87`; `calendar_service.go:36-38` (route `router.go:485`)
**Description.** `GET /calendar_events/:id` (bare `protected`) only scopes by tenant; it performs no check that the event's `ContextType`/`ContextID` includes the caller. `ListEvents` correctly restricts to the caller's User context or enrolled Course; `GetEvent` has no equivalent gate.
**Impact.** Any authenticated user can enumerate event IDs in their tenant and read other users' personal calendar entries — title, description, location/address (PII; a minor's or teacher's whereabouts).
**Recommendation.** After loading, enforce a context check (User→`ContextID == caller` or observer/admin; Course/Group→enrolled/member; else 404), reusing the injected `ResourceAuthorizer`.
**Verification:** 1-vote.

---

### LOW

| ID | Title | Location | Verification |
|----|-------|----------|--------------|
| SEC-091 | Instructor/CourseRole guards on routes lacking a `:course_id` param silently fail closed (fragile wiring; effectively admin-only) | `router.go:665`; `routes_quiz_extensions.go:27,50-51` | 1-vote |
| SEC-092 | Pending MFA/password-reset JWTs not invalidated after successful step-up (5-min replay window; pending token in `?t=` redirect URL) | `handlers/mfa.go:281-309`; `users.go:312-360` | 1-vote |
| SEC-093 | Pending JWTs omit `iss`/`aud` validation the session path enforces (deviation from "every JWT carries iss/aud") | `auth/mfa_pending.go:64-82`; `password_reset_pending.go:69-87` | 1-vote |
| SEC-094 | Password-reset token stored plaintext (not hashed); COPPA verification token shares the pattern | `service/user_service.go:135-142` | 1-vote |
| SEC-095 | Parental-consent verification tokens never expire (`ExpiresAt` left nil; nil treated as "never") | `coppa_service.go:45-118` | 1-vote |
| SEC-096 | `Submission.Grade` accepts NaN/Inf/unbounded values (poisons whole-course aggregates); also affects `BulkGrade` | `submission_service.go:217-280` | 1-vote |
| SEC-097 | Promised `SafeDialer` never implemented — every guarded fetch is TOCTOU/DNS-rebinding vulnerable | `internal/security/ssrf.go:51` | 1-vote |
| SEC-098 | Webhook communication-channel address accepted without SSRF/scheme validation at creation time | `notification_delivery.go:150` | 1-vote |
| SEC-099 | Conversation reply `Body` stored without `SanitizeHTML` (inconsistent with bulk-send path; current UI auto-escapes) | `conversations.go:347` | 1-vote |
| SEC-100 | Submission comment text stored without `SanitizeHTML` (current UI auto-escapes; API serves raw) | `submissions.go:398` | 1-vote |
| SEC-101 | Frontend DOMPurify allowlist permits iframe/object/embed/form/style/data: for all rich content (sole gate for import-only content) | `web/src/components/RichContentViewer.jsx:17` | 1-vote |
| SEC-102 | Quiz stimulus `Content` persisted without `SanitizeHTML` on create+update (one DOMPurify-protected render today) | `quiz_stimuli.go:82,116` | 1-vote |
| SEC-103 | Portfolio export allows `javascript:`/`data:` scheme URLs in href/src (`escapeHTML` does not block schemes) | `portfolio_service.go:633,656,659,662,559,555` | 1-vote |
| SEC-104 | Quiz answer-feedback fields (correct/incorrect/neutral_comments) skip `SanitizeHTML` on create AND update (REST path) | `quiz_questions.go:109-111,166-172`; `quiz_item_banks.go:206-208,268-274` | 1-vote |
| SEC-105 | S3 file downloads bypass nosniff + Content-Disposition (presigned GET sets neither) — code-level confirmation of SEC-018 | `storage/s3.go:302-309`; `files.go:296-302` | 1-vote |
| SEC-106 | Stored XSS via `javascript:` href on collaboration documents (`href={collab.url}`, no validation) | `web/src/pages/CollaborationsPage.jsx:241`; `collaborations.go:94` | 1-vote |
| SEC-107 | Two additional instructor pages render QTI-imported `question_text` raw (no DOMPurify) | `ItemAnalysisPage.jsx:125`; `StimulusEditorPage.jsx:202` | 1-vote |
| SEC-108 | Upload `Content-Type` is attacker-controlled and stored unvalidated for unknown extensions (precondition for SEC-018) | `files.go:142`; `file_service.go:119-137` | 1-vote |
| SEC-109 | `content_migrations` upload writes to a path built from the client filename (stdlib `filepath.Base` mitigates today) | `content_migrations.go:117-124` | 1-vote |
| SEC-110 | `content_imports` upload path built from client filename; UUID prefix gives false safety (stdlib mitigates today) | `content_import.go:66-69` | 1-vote |
| SEC-111 | `POST /logout` has no CSRF protection (cross-site forced logout) | `routes_public.go:28` | 1-vote |
| SEC-112 | Trusted-proxy not configured; `X-Forwarded-Proto` honored from any client for HSTS/protocol logic | `cmd/server/main.go:1014`; `middleware/security.go:55` | 1-vote |
| SEC-113 | SAML `SubjectConfirmationData` `Recipient`/`NotOnOrAfter` parsed but never validated | `auth/saml.go:322-331,578-648` | 1-vote |
| SEC-114 | SAML RelayState used directly as post-login redirect (open redirect; leaks mfa_pending token off-site) | `auth/saml.go:719-751` | 1-vote |
| SEC-115 | SAML signature cert lookup hardcoded to `account_id=1`, uses first-parseable cert without matching assertion Issuer (multi-IdP confusion) | `auth/saml.go:884-906,761-787` | 1-vote |
| SEC-116 | Single-process in-memory SAML replay cache leaves multi-pod deployments replay-able | `auth/saml_replay_cache.go:19-78` | 1-vote |
| SEC-117 | SIS import bcrypt-hashes a password per CSV row with no row-count cap (CPU exhaustion) | `sis_import_service.go:122-228` | 1-vote |
| SEC-118 | Hand-rolled GraphQL parser has unbounded recursion → unrecoverable stack-overflow crash | `internal/graphql/schema.go:79-137` | 3-lens (conf 2 / ref 1) |
| SEC-119 | Rubric assessment score summed from attacker-supplied per-criterion points, unvalidated against rubric max | `rubric_service.go:169-217` | 1-vote |
| SEC-120 | Manual badge award does not verify the target user's tenant | `gamification/badge_service.go:217-235` | 1-vote |
| SEC-121 | `AnswerQuestion` does not verify the question belongs to the submission's quiz (junk rows; no grade-inflation path found) | `quiz_attempts_service.go:305-347` | 1-vote |
| SEC-122 | Assignment publish toggle sets `workflow_state` directly, bypassing the state machine | `assignments.go:242-249` | 1-vote |
| SEC-123 | Discussion entry V1 Update/Delete routes use wrong path param (`:id` vs handler `:entry_id`) — fail-closed today, masks unscoped service | `router.go:374`; `discussion_entries.go:87` | 1-vote |
| SEC-124 | WikiPageRevision Get/Revert resolve by revision_id with no parent-tie/tenant scope (handler currently unrouted) | `wiki_page_revisions.go:59`; `wiki_page_revision.go:23` | 1-vote |
| SEC-125 | No `govulncheck` gate in CI — dependency/stdlib advisories not caught at merge | `.github/workflows/ci.yml:9-102` | 1-vote |
| SEC-126 | `golang.org/x/crypto@v0.51.0` has 14 open SSH advisories (ssh subpackages not on any call path; hygiene only) | `go.mod:27` | 1-vote |
| SEC-127 | FERPA `GetExportRequest` fetches by id without tenant scope before the ownership check | `ferpa.go:157-179` | 1-vote |
| SEC-128 | Peer review submit accepts unbounded score in addition to the ownership gap (duplicate surface of SEC-029, score-validation angle) | `peer_reviews.go:77-97`; `peer_review_service.go:115-130` | 1-vote |
| SEC-129 | Manual badge award tenant check (gamification economy surface; duplicate of SEC-120) | `gamification/badge_service.go:217-235` | 1-vote |
| SEC-130 | Peer reviews `ListPeerReviews`: missing assignment→course parent-tie (same-tenant cross-course enumeration) | `peer_reviews.go:43-59` | 1-vote |
| SEC-131 | Discussion entry `UpdateEntry`/`DeleteEntry` lack ownership check (latent IDOR masked by param-name mismatch) | `discussion_entries.go:86-127` | 1-vote |
| SEC-132 | Same-tenant cross-portfolio IDOR: portfolio section/artifact write+delete skip the parent-tie `ReorderSections` enforces | `portfolio.go:411-490,566-662`; `portfolio_service.go:173-241` | 1-vote |
| SEC-133 | Local storage backend joins key without confining to basePath (no current escaping caller) | `internal/storage/local.go:22` | 1-vote |
| SEC-134 | Global Fiber BodyLimit (100 MB) contradicts the documented 5 GB net and silently caps the per-tenant upload limit | `cmd/server/main.go:1024`; `upload_size.go:32,38`; `catalog.go:306` | 1-vote |
| SEC-135 | `content_migrations` upload route not behind `EnforceUploadSize` (only global 100 MB + rate limiter) | `router.go:508`; `content_migrations.go:118-124` | 1-vote |
| SEC-136 | Dev `docker-compose.yml` ships a weak hardcoded `JWT_SECRET` (prod guarded by fatal default-value check) | `docker-compose.yml:26` | 1-vote |
| SEC-137 | Assignment override `Create` lacks the parent-tie enforced on Get/Update/Delete | `assignment_overrides.go:91-139` | 1-vote |

### INFO

| ID | Title | Location | Verification |
|----|-------|----------|--------------|
| SEC-138 | No SQL injection vectors found in the data-access layer (verified twice, end to end — placeholders + const SQL throughout) | `internal/repository/postgres/` (whole package) | 1-vote ×2 |
| SEC-139 | No tracked secrets, `.env`, or private keys in the repo or git history | `.gitignore:12` (whole tree) | 1-vote |
| SEC-140 | Dev seed/test fixtures contain shared static passwords (intentional, dev-only, not in prod image) | `cmd/seedtestdata/main.go:40`; `test/dex/config.yaml:38` | 1-vote |
| SEC-141 | No secrets logged: TOTP/OIDC/LDAP secrets never written to slog/log/fmt; plaintext TOTP returned only to the enrolling user | `handlers/mfa.go:107-134`; `internal/auth/*.go` | 1-vote |
| SEC-142 | Frontend prod deps several minors behind (DOMPurify 3.4.3 vs 3.4.7) — hygiene; no open advisory | `web/package.json:36` | 1-vote |
| SEC-143 | `ResourceAuthorizer.isAdmin` recognizes only `"admin"`, not `"super_admin"` (fail-secure under-grant; inconsistent with middleware) | `handlers/authz.go:26` | 1-vote |
| SEC-144 | `CSPNonce` middleware (nonce + strict-dynamic CSP) never mounted — documented hardening does not ship (static CSP is still strict) | `middleware/csp.go:37` | 1-vote |
| SEC-145 | `style-src 'unsafe-inline'` in the enforced production CSP (Tailwind; scripts locked down) | `middleware/security.go:21` | 1-vote |
| SEC-146 | SAML Response `Destination` never validated against the SP ACS URL (defense-in-depth) | `auth/saml.go:286` | 1-vote |
| SEC-147 | Custom gradebook column cell content stored without `SanitizeHTML` (teacher-authored, no HTML render sink located) | `custom_gradebook_columns.go:208` | 1-vote |
| SEC-148 | Standalone conference/collaboration object routes default-allow when `ContextType != "Course"` (currently unreachable) | `conferences.go:133-336`; `collaborations.go:106-216` | 1-vote |
| SEC-149 | Unwired `ContentExportHandler.DownloadExport` has an unsanitized `:export_id` in a filesystem path (dead code) | `content_export.go:99-119` | 1-vote |
| SEC-150 | Upload MIME validation trusts client Content-Type, no magic-byte sniffing (nosniff + blocklist hold today) | `file_service.go:119` | 1-vote |
| SEC-151 | GraphQL ID arguments accept lax `Sscanf` prefix parsing (`"12abc"`→12; no SQLi consequence) | `resolver.go:505` | 1-vote |
| SEC-152 | GraphQL resolver errors return verbatim wrapped GORM/DB error text to the client (information disclosure) | `resolver.go:64` | 1-vote |
| SEC-153 | Trusted IMSCC import intentionally bypasses the SVG/HTML upload blocklist (safety rests on attachment-disposition + nosniff) | `file_service.go:236` | 1-vote |

---

## 4. Findings Reviewed and Excluded as False Positives

These claims were investigated and refuted (or materially downgraded) during verification — demonstrating that not every finder hypothesis survives scrutiny.

| Title | Claimed severity | Why refuted |
|-------|------------------|-------------|
| SQL injection via dynamic table name in user-deletion path | Info | `updatePIIColumns(tx, table, whereSQL,...)` — all 6 call sites pass compile-time string literals for `table` and `whereSQL`; no request input reaches the dynamic name. Confirmed safe. |
| LocalBackend storage methods do not defensively clean the key before joining basePath | Info | Code citation accurate, but no reachable traversal: every key reaching a backend is built server-side as `ContextType/<id>/<uuid>/<sanitizeFilename(name)>` (`/` stripped). Defense-in-depth note, not a vuln (retained as SEC-133 Low). |
| `.env.example` contains real credentials | Info | Full read confirms only placeholders/empty values (`JWT_SECRET=change-me-in-production`, empty S3/SMTP). Clean by design. |
| `golang.org/x/net@v0.54.0` ships 5 open html-parser advisories feeding the bluemonday sanitizer | High | Dependency facts confirmed (govulncheck reproduces GO-2026-5025/5027/5028/5029/5030, fixed v0.55.0) and the package IS reachable via bluemonday→`html.NewTokenizer`. **Severity overstated** — none of the advisories are on the `SanitizeHTML` call path's exploitable surface; treat as a hygiene bump (track via SEC-125 CI gate). |
| Go toolchain pinned to 1.25 exposes 4 reachable stdlib advisories (html/template, net/http, net) | High | Infra pins all confirmed and the cited call paths exist, but the specific advisories were not demonstrated exploitable on the rendered paths (`html/template` escaping is the documented F-056 hardening, not a bypass). Downgraded to a toolchain-currency/CI-gate item, not a confirmed live vuln. |
| Rate-limit store defaults to in-memory; multi-pod splits the budget | Low | Premise false: `RedisStore` (atomic Lua ZSET sliding window) IS implemented and wired at boot when `REDIS_URL` is set (`main.go:164-172`). The "pending 13.6.A backend" is shipped. |
| QTI/IMSCC import stores answer/feedback comment fields unsanitized (broader than question_text) | Medium | The literal copy claim holds, but the only present renders of those fields are React-escaped form inputs — no live XSS. The genuine, narrower gap (REST create/update skipping feedback-field sanitize) is retained as SEC-104 (Low). |
| Detached `innerHTML` parse of unsanitized HTML in `crossCourseLinks` may trigger `img onerror` | Low | No cross-user trust boundary — `detectCrossCourseLinks` runs at save time on the editing user's own in-progress input; downstream render treats content as text. Not exploitable. |
| Access-token auth path leaves `account_id` unset for `AccountID==0` users, collapsing GraphQL tenancy | Medium | Asymmetry in `auth.go:194` is real, but the exploit requires an `AccountID==0` user, which the system does not produce for request-facing access-token holders; the claimed cross-tenant collapse is not reachable. |

---

## 5. Areas Warranting Manual Follow-Up

The completeness critic identified attack surface that the confirmed findings did **not** fully cover. These are not yet confirmed vulnerabilities — they are prioritized review gaps.

1. **LTI AGS line-item grade write/read (`PostScore`, `GetResults`, `UpdateLineItem`, `GetLineItem`)** — `lti_line_item.go:23 FindByID(ctx, id)` takes no accountID (only `Delete` got the F-012 widening); handlers (`lti.go:597,666,499`) never assert `lineItem.CourseID == :course_id`. `PostScore` syncs into the gradebook and the `userId` in the score body is attacker-controlled. **Strongly suspected cross-tenant IDOR + grade-injection (~1,350 LOC essentially unreviewed).**
2. **LTI NRPS roster export (`GetMemberships`)** — `lti_nrps_service.go:30` calls `enrollmentRepo.ListByCourseID(ctx, courseID, 0, params)` (accountID hardcoded 0); a bulk-PII export endpoint with no confirmed tenant tie.
3. **AI Assist endpoint authz + cost amplification** — `POST /ai_assist/:action` is in the generic `protected` group with only `AIAssistRateLimit()` and no role gate; `input.Text` has no length cap before billing api.anthropic.com. Any student can drive paid API calls (financial DoS).
4. **LTI 1.1 `SharedSecret` (and LTI tool ConsumerKey/private key) stored plaintext** — `context_external_tool.go:15` has `json:"-"` but no encryption; violates the secretbox invariant (same class as SEC-040/SEC-056). Confirm LTI 1.3 tool private keys too.
5. **Audit log attributes masquerading-admin writes to the victim** — `audit_writes.go:47` records only `c.Locals('user_id')` (the impersonated target during masquerade), never the admin actor. Compounds SEC-036: every destructive write under masquerade is logged under the victim's identity (non-repudiation failure).
6. **Outbound webhook payload integrity** — `notification_delivery_service.go` does not HMAC-sign the webhook body; receivers cannot authenticate Paper LMS as sender. Compounds the SSRF egress (SEC-044/SEC-098).
7. **Mass-assignment via repo `.Save()`/`.Updates()` of body-bound structs** — ~30 `Save()` sites write every column; verify Update handlers do not re-bind privilege fields (`account_id`, `user_id`/owner, `points_possible`, `workflow_state`) from the JSON body.
8. **WebAuthn/passkey verification (RP ID, origin allow-list, challenge single-use, sign-count regression, user-handle binding)** — passkey skips MFA by design, so its crypto verification is the entire boundary, yet only the cookie `Secure` flag (SEC-037) was reviewed.
9. **Scheduler / background jobs with `accountID=0`** — leaderboard-snapshot, FERPA deletion, gamification rule jobs run off the request path where the tenant discipline can degrade to `0`. Verify no cross-tenant aggregation.
10. **Pairing-code entropy + observer-link tenant isolation** — `GeneratePairingCodeString`/`Redeem` is the parent/observer linkage primitive (full grade/PII access). Verify code entropy, `Redeem` brute-force rate-limiting, and that `LinkObserverToStudent` (which uses `accountID=0`) enforces same-tenant.

---

## 6. Code Organization & Structure

### 6.0 Lead concern: no CI security scanning (HIGH)

The single most important organizational gap for public release: **the entire repo has zero security-scanning automation.** A full search for `gosec|govulncheck|semgrep|codeql|trivy|snyk|npm audit|dependabot|grype|osv-scanner` returns no workflow, config, or Make target. The two workflows are `ci.yml` (lint/test/build/docker/deploy) and `a11y.yml` (axe-core). `npm audit` is explicitly **disabled** (`--no-audit` on every install). For a multi-tenant K-12 LMS that auto-deploys to production on every push to `main`, the gating CI never checks Go module/stdlib CVEs, the npm tree, Go source for insecure patterns, or built images.

**Recommendation (P0):** Add a `security` workflow (PR + push + weekly `schedule`) with: (1) **`govulncheck ./...`** — blocking (would have caught every Go advisory in this report); (2) **`gosec`** via `.golangci.yml` or standalone; (3) **`npm audit --audit-level=high`** (or `osv-scanner`) in `web/` — start non-blocking, then promote; (4) **Trivy `fs` + `image`** scans wired into the existing `docker` job; (5) optionally **CodeQL** (Go + JS). Document the gate in the project guide alongside the others.

### 6.1 Layering

The intended architecture is `handler → service → repository (interface) → postgres adapter`, but it is violated in three patterns:

- **Repo-only handlers (HIGH):** Five handlers (`quizzes.go`, `sections.go`, `grading_standards.go`, `notification_delivery.go`, `accounts.go`) are constructed with only a repository and implement full CRUD + business logic directly. `quizzes.go` is the worst — it inlines `SanitizeHTML`, model building, tenant scoping, and `quizRepo.Create/Update/Delete` even though a full `quiz_*_service.go` cluster exists and is never used.
- **Mixed-layer handler deps (HIGH):** ~26 handlers inject repositories alongside services and issue writes directly (`submissions.go` holds 5 raw repos and calls `commentRepo.Create`; `learning_outcomes.go` calls `alignmentRepo.Create`). Rule to enforce: **handlers may hold `*service.X` only; any `repository.X` in a handler constructor is a review flag.**
- **Services holding raw `*gorm.DB` (HIGH):** 8 service files import gorm; 5 run real queries in the business layer (`enrollment_term_service.go`, `appointment_group_service.go`, `user_deletion_service.go` entirely raw-SQL, `oneroster_service.go`, `sis_import_service.go`).
- **DIP / abstraction leaks (MEDIUM–LOW):** 14 services depend on concrete `*postgres.XxxRepository`; 4 handlers consume persistence types (`postgres.AuditLogFilter`) at the HTTP boundary; 3 handlers import `gorm` solely to branch on `gorm.ErrRecordNotFound` (should be a `repository.ErrNotFound` sentinel).

**Target:** `handler → service interface → repository interface → postgres adapter`, with a CI lint failing if `internal/api/v1/handlers` imports `internal/repository` for anything beyond shared types.

### 6.2 God files

| File | Size | Problem | Recommendation |
|------|------|---------|----------------|
| `web/src/services/api.js` | 2,899 LOC | ~460 flat verb-named CRUD methods on one object | Split into `api/<domain>.js` + `api/client.js` core; re-export aggregate from `api/index.js` |
| `cmd/server/main.go` | 1,121 LOC | 132 repos + 79 services + 88 handlers wired inline | Extract `buildRepositories`/`buildServices`/`buildHandlers`/`registerEmitCallbacks`/`runStartupBackfills` |
| `internal/api/v1/router.go` `Register()` | 646 LOC | ~333 routes in one method | Extract one `registerXxxRoutes` per comment section (follow `registerSuperAdminRoutes`) |
| `web/src/pages/GradebookPage.jsx` | 1,864 LOC (888-LOC component, 30 useState) | 12 inline sub-components | Move dialogs/grid to `gradebook/`, lift state into `useGradebookState` |
| `internal/service/imscc_parser.go` | 1,983 LOC, 38 funcs | Owns every Canvas entity importer | Split into an `imscc/` package |
| `internal/service/portfolio_service.go` | 873 LOC | CRUD tangled with HTML/CSS/PDF rendering | Extract `portfolio_render.go`; themes as `embed.FS` |
| `internal/api/v1/handlers/portfolio.go` | 894 LOC, 30 funcs | 7 sub-resources + serializers | Split per sub-resource + `portfolio_dto.go` |
| `internal/auth/saml.go` `HandleACS()` | 244 LOC (security-critical) | XSW defense in a long branching handler | Extract `validateSignedResponse`/`extractAndValidateAttributes`/`resolveOrProvisionUser` (under security review) |

Also flagged: `imscc_exporter.go` `ExportCourse()` (317 LOC), `gamification.go` (782 LOC, 5 sub-domains), `users.go` (819 LOC, auth+profile+masquerade+COPPA), and a cluster of 1000+ LOC JSX pages (`PortfolioEditorPage`, `DiscussionTopicPageV2`, `ModulesPage`, `RichContentEditor`, `AssignmentPage`).

### 6.3 Dead code & duplication

- **Three complete-but-unmounted feature stacks (HIGH):** Planner, Wiki Page Revisions, and Comment Bank each have handler + repo + service but no router registration or `main.go` construction (~1,240 LOC + models/migrations). `deadcode` flags every method unreachable. **Decide: wire (+ routing test) or delete as a unit.**
- **IMSCC export stack ~1,800 LOC is dead (HIGH):** Import is live; `ContentExportHandler`/`NewIMSCCExporter` and ~20 `write*XML` helpers are unreachable (only the roundtrip test references them). No IMSCC export route exists.
- `DiscussionV2Service` has 10 unused CRUD methods duplicating V1 (MEDIUM).
- `FERPAService.LogPIIAccess` duplicates `AuditService.LogPIIAccess`; the FERPA copy is dead (a code comment even acknowledges it) (MEDIUM).
- `workflow_state` transition validators built for 4 entities, only Course wired — 3 dead (intentional scaffolding) (MEDIUM).
- Mastery Calculator abstraction unconstructed (`Method()` dead on all 6 implementations) (LOW).
- ~210 orphaned functions via `deadcode -test` intersection; run it advisory-in-CI and triage (LOW).
- 59 stale agent worktrees (3.6 GB) under `.claude/worktrees` pollute local tooling (gitignored, so not in-repo) (LOW).
- Mixed param-ID parsing idioms (`c.ParamsInt` ×542 vs `strconv.ParseUint`/`Atoi` ×46) — add one `paramUint` helper (LOW).

### 6.4 Naming

The repository layer is the sole outlier across the three parallel layers:

- **Constructor/struct mismatch (MEDIUM):** 133 of 140 repo structs end in `Repo` but **100%** of constructors are `NewXxxRepository`; the 7 stragglers are named `XxxRepository`. Standardize the trio: interface `XxxRepository`, struct `xxxRepo`, constructor `NewXxxRepository`.
- 22 repo structs exported vs 118 unexported (MEDIUM) — make all concrete structs unexported.
- 29 constructors return concrete `*XxxRepo` instead of the interface (MEDIUM).
- 11–18 repo interfaces declared inside the `postgres` adapter package instead of the central `repository` `*_interfaces.go` (MEDIUM).
- FERPA acronym casing split (`FERPAService` vs `FerpaClassification`) vs the all-caps LTI/SIS/OIDC/COPPA precedent (LOW).
- `outcome_alignment_repo.go` uses a `_repo` suffix; all 119 peers use bare-entity names (LOW).
- Handler filenames mix plural (CRUD) vs singular (feature) — defensible; document the rule (INFO).

### 6.5 Package boundaries

- `internal/service` is a **146-file flat package** mixing ~25 domains (HIGH); only `gamification/` and `settings/` are carved out. Intra-service coupling is very low (only 14 sibling-service field refs), so a split is **low-risk** (INFO supporting finding).
- `internal/api/v1/handlers` is a **99-file flat package** (HIGH); the Router already groups it into ~25 comment sections — promote those into subpackages.
- 20 of ~92 handlers bypass the service layer (HIGH — same root as 6.1).
- Repository interfaces split between `repository/*_interfaces.go` (30) and inside `postgres/` (11) (MEDIUM).
- imscc/qti logic duplicated across the dedicated `internal/qti` package and 8 service files (~5,000 LOC) (MEDIUM).
- `cmd/server/main.go` wiring monolith (MEDIUM — same as 6.2).
- **Correctly flat, do NOT split:** `domain/models` (121 files, shared-type vocabulary — splitting causes import cycles) and `internal/repository/postgres` (consistent one-adapter-per-aggregate; mirror any future contract split).
- **Healthy reference patterns:** `internal/{obs,config,security,settingsctx,auth,storage,scheduler}` and `service/gamification/wiring/` — use these as the in-repo model when sub-grouping.

**Proposed target tree (domains):** `assessment`, `grading`, `content`, `interchange` (consolidate imscc/qti into `internal/qti`), `identity`, `rostering`, `compliance`, `communication`, `gamification` — each a service subpkg + handler subpkg + repo-interface group, mirroring the Router's existing comment sections. **Sequence:** (1) lift the 11 misplaced repo interfaces (cheap); (2) route the 20 bypass handlers through services + add the no-repo-import lint; (3) consolidate imscc/qti parse logic; (4) extract per-domain wiring funcs to shrink `main.go`; (5) move files into domain subpackages one domain at a time, validating each step against the schema-parity and acronym CI gates.

### 6.6 Testing

- **Frontend vitest (35 test files) never runs in CI (HIGH)** — `frontend-build` runs only `npm run build`; no `npm run test` step. The tests exist and pass locally; wire them to a required check.
- **Repository/postgres layer: 120 implementations, 2 test files (HIGH)** — the tenant-isolation `WHERE`-clause core (trailing-accountID, child-scoping, polymorphic default-deny, `accountID==0` escape hatch) is largely unverified. Use the existing `PARITY_DB_URL` harness to add table-driven cross-tenant tests for the highest-risk repos first.
- **Service layer: ~33 of ~95 tested; auth/LTI/FERPA/COPPA/OAuth2 services have zero unit tests (HIGH)** — prioritize `ferpa_service`, `coppa_service`, the LTI/OAuth2 trio (pure logic, 66 testify mocks already exist).
- **No coverage threshold gate; golangci-lint is `continue-on-error` (MEDIUM)** — add a patch-coverage gate and remove `continue-on-error`.
- Gamification wallet API surface + dispatcher cooldown→award→ledger chain lack direct tests (MEDIUM).
- Handler test coverage is the **strongest layer** (34/99 files, 29 referencing tenant/cross-tenant/404 semantics) — backfill enrollments/gradebook/files/oauth2/ferpa next (INFO).
- Orphaned mocks (folder/conference/collaboration/conversation/appointment_group) map exactly to untested consumers; `conversation_service` is COPPA-adjacent and should jump the queue (LOW).
- GraphQL and storage packages have zero tests despite being externally reachable (LOW).

### 6.7 Build / CI

- golangci-lint is **advisory-only** (`continue-on-error: true`) and pinned to floating `latest`; **no `.golangci.yml`** exists (HIGH). Add a checked-in config (errcheck, govet, staticcheck, ineffassign, unused, bodyclose, gosec), pin the action to a SHA, and remove `continue-on-error` once baseline is clean.
- **Production Docker images run as root** (no `USER` directive in either Dockerfile); compose sets no `security_opt`/`cap_drop`/`read_only`/resource limits (HIGH). Add non-root users, `no-new-privileges`, `cap_drop: [ALL]`, `read_only` + tmpfs, and resource limits.
- Frontend lint + 35 tests never run in CI (HIGH).
- Frontend Dockerfile uses `npm install` (mutates lockfile) instead of `npm ci` (MEDIUM).
- **No `.dockerignore`** — full repo (incl. `.env`, ~140 MB prebuilt binaries, `.git`, `.claude/worktrees`) sent to build context; secrets land in the intermediate builder layer (MEDIUM).
- All GitHub Actions pinned to mutable major tags, not SHAs — including `appleboy/ssh-action@v1` which holds the prod SSH key (MEDIUM).
- No Dependabot/Renovate (MEDIUM).
- Production deploy is fully automatic on push to `main` with no environment/approval gate, and does not depend on `frontend-build` or lint (MEDIUM).
- Node version drift (CI Node 22 vs Dockerfile Node 20) (LOW); base images pinned to floating tags not digests (LOW); `go vet` sits in the non-blocking, non-deploy-`needs` lint job (LOW).

### 6.8 Public-repo readiness

- **Shipping source references gitignored `CLAUDE.md`** in 11 tracked files (Go, SQL migration, tests), plus a `[[feedback_...]]` wiki-link exposing the internal KB convention (HIGH). Sweep + repoint to `PROJECT.md`/`CONTRIBUTING.md`; add a CI grep gate.
- **Internal session-handoff doc ships** (`docs/status/2026-05-15-phase-10-handoff.md`) leaking local absolute paths, PIDs, `/tmp` log paths, working-tree state, and a verbatim agent prompt (HIGH). Gitignore it.
- Local absolute path to a private plan file in `cmd/server/main.go:1110` (MEDIUM); hardcoded personal path in `imscc_roundtrip_test.go:66` (MEDIUM); internal AI-research/PRD docs under `docs/research/gamification-2026-05/` (MEDIUM); public `SECURITY.md` links to an audit doc leaking branch state + `CLAUDE.md` refs (MEDIUM).
- README quickstart points to the wrong port (`localhost:8080` vs published 80/443) (LOW); `STALE_COLUMNS.md` generated artifact in repo root (LOW); unexplained Wave/Phase vocabulary in shipping docs (INFO).
- **Healthy:** no tracked binaries, `.env` untracked, thorough secret-free `.env.example`, MIT LICENSE, real `SECURITY.md` disclosure SLA, low TODO density, no debug prints in request paths (INFO).

---

## 7. Prioritized Remediation Roadmap

### P0 — Now (active cross-tenant breaches and credential exposure)

- [ ] **SEC-005** Tenant-scope SIS CSV exports (full-deployment PII leak).
- [ ] **SEC-028** Add `RequireAccountAdmin(:account_id)` middleware (fails closed) and apply to all `/accounts/:account_id/*` routes — fixes the root cause behind SEC-005, SEC-019/SEC-020/SEC-021/SEC-022/SEC-023/SEC-024/SEC-025/SEC-026/SEC-086.
- [ ] **SEC-001 / SEC-006 / SEC-033 / SEC-034 / SEC-035** Tenant-scope and add enrollment/role authz to GraphQL + the REST `scope=all` path; reject `accountID==0` from request paths.
- [ ] **SEC-002 / SEC-003 / SEC-004 / SEC-052 / SEC-070** Add owner/instructor/observer + parent-tie guards to the submissions/comments/annotations family.
- [ ] **SEC-031** Thread accountID through the entire student-accommodation stack (disability/IEP data).
- [ ] **SEC-040 / SEC-056** Encrypt OneRoster + OAuth2 developer-key secrets at rest (secretbox/hash); constant-time compare; backfill rows.
- [ ] **SEC-037** Centralize session-cookie minting with `Secure` on all federated/MFA/passkey paths.
- [ ] **SEC-044** Set `CheckRedirect` (re-validate per hop) on production HTTP clients (webhook/CAS/OneRoster/OIDC).
- [ ] **SEC-007..SEC-016, SEC-029, SEC-030, SEC-050, SEC-051, SEC-053, SEC-054, SEC-061** Apply the F-013 parent-tie + tenant scope to quiz questions/groups, commons publish, speedgrader, quiz statistics, discussions V2, rubric assessments, mastery paths, post/hide grades, peer-review submit, global announcements, checkpoints, course paces, DPAs, COPPA consent, module items.
- [ ] **SEC-017** Add authz + tenant scope to `/folders/:folder_id/files`.

### P1 — Before public release

- [ ] **SEC-018 / SEC-105 / SEC-108** Fix S3 presigned download disposition/nosniff; validate stored Content-Type at upload.
- [ ] **SEC-038 / SEC-039** SAML: fail closed on missing `<Conditions>`; add `InResponseTo`/AuthnRequest-ID binding (and SEC-113/SEC-114/SEC-115/SEC-146 hardening).
- [ ] **SEC-041 / SEC-042 / SEC-043 / SEC-058..SEC-062, SEC-104, SEC-106, SEC-107** Close stored-XSS: portfolio export + public-page URLs, QTI/IMSCC import sanitize, all `question_text` render sinks, collaboration URLs.
- [ ] **SEC-045 / SEC-089 / SEC-073 / SEC-074 / SEC-118 / SEC-046** DoS hardening: zip decompression caps, GraphQL depth/complexity/perPage/rate limits, parser recursion cap.
- [ ] **SEC-036** Add role-precedence check to masquerade; **(Follow-up #5)** capture the true actor in audit during masquerade.
- [ ] **SEC-055 / SEC-057** Enforce super_admin MFA under `required_admin`; enforce `JWT_SECRET` length in production.
- [ ] **SEC-077 / SEC-078 / SEC-098 / SEC-117** Add rate limits / input caps to OAuth2 token, bulk endpoints, webhook creation, SIS import.
- [ ] **Org-P0:** Add the CI security workflow (`govulncheck` blocking, `gosec`, `npm audit`/`osv-scanner`, Trivy) — Section 6.0; this also resolves SEC-125/SEC-126/SEC-142.
- [ ] **Org-HIGH:** Drop root in production images; add `.dockerignore`; wire frontend lint+tests into CI; remove `continue-on-error` on golangci-lint with a checked-in `.golangci.yml`.
- [ ] **Public-readiness HIGH:** Sweep tracked source for `CLAUDE.md`/`[[...]]`; gitignore the session-handoff and AI-research docs; add a CI grep gate for `/Users/` and `.claude/`.
- [ ] **Follow-up #1–#4:** Manually review and remediate LTI AGS line-item IDOR, LTI NRPS roster export, AI Assist authz/cost, and LTI 1.1 `SharedSecret` plaintext.

### P2 — Follow-up (hygiene, defense-in-depth, structure)

- [ ] **SEC-066/SEC-067/SEC-032/SEC-087/SEC-088/SEC-090/SEC-127/SEC-130/SEC-132/SEC-137** Remaining medium IDOR/authz gaps and tenant-blind admin branches.
- [ ] **SEC-081/SEC-082/SEC-096/SEC-119/SEC-120/SEC-121/SEC-122** Gamification idempotency, grade-value validation, state-machine consistency.
- [ ] **SEC-092/SEC-093/SEC-094/SEC-095/SEC-097/SEC-111/SEC-112/SEC-116** Token lifecycle, SafeDialer (TOCTOU), logout CSRF, trusted-proxy config, SAML replay cache (Redis).
- [ ] **SEC-083 / Follow-up #9/#10** FERPA deletion completeness (PII registry), background-job tenant scope, pairing-code entropy/rate-limit.
- [ ] **SEC-144/SEC-145/SEC-101** CSP: mount `CSPNonce` or delete it; tighten DOMPurify allowlist by trust tier; drop `style-src 'unsafe-inline'`.
- [ ] **Org refactors:** layering invariant + no-repo-import lint; split the three flat packages by domain; remove/wire the dead feature stacks (Planner/Wiki/Comment Bank/IMSCC export); standardize repo naming; lift the 11 misplaced repo interfaces; backfill repository + auth/LTI/FERPA/COPPA service tests; SHA-pin Actions + add Dependabot + a production deploy approval gate.

---

## 8. Appendix

### A. Architecture & attack-surface map

**Request lifecycle (`cmd/server/main.go:1014-1106`).** Fiber global middleware order: `fiberrecover` → `RequestID` → `Observability` → `SecurityHeaders` (CSP/HSTS/XFO=DENY/nosniff) → `InputValidation` (10 MB non-multipart cap) → logger → `cors` (origin = `cfg.FrontendURL`, `AllowCredentials:true`). `router.Register` then mounts `/api/v1` with `PaginationParams()`; public routes first, then the `protected` group adds `Protected()` + `CSRFProtection()` + a single `AuditWrites` mount (~333 write routes). A dev-only `/_test/panic` exists (gated to development). **Note:** the global `BodyLimit` is actually 100 MB (`main.go:1024`), contradicting the documented 5 GB net (SEC-134) — relevant to the import-DoS findings.

**Auth / session / tenant context (`middleware/auth.go`).** `Protected()` reads `Authorization: Bearer` first, else the `paper_session` httpOnly cookie. JWT path pins `WithIssuer`/`WithAudience`, checks `tokenBlacklist`, rejects `purpose:`-prefixed jti, and sets Locals incl. `account_id` and masquerade Locals. Access-token (PAT/OAuth2) path sets `token_scopes`. `is_admin`/`is_super_admin` from the JWT are soft hints — `permissions.go` re-fetches the user row at every gated route. Tenant `account_id` is threaded as the trailing `accountID` in repo calls; `accountID==0` is the documented auth-internal escape hatch (abused in several findings).

**RBAC guards (`middleware/permissions.go`).** `RequireAdmin` (role + own-tenant tie, super_admin/root bypass), `RequireSuperAdmin`, `RequireEnrolled`, `RequireInstructor`, `RequireSelfOrAdmin`, `RequireCourseRole`. **Structurally notable:** `RequireAdmin` never reads the URL `:account_id` (SEC-028); the `isAdmin` branch ignores course/tenant (SEC-066/SEC-067); LTI OIDC-login + launch ride a separate auth-only group **without CSRF**; CSRF is skipped for Bearer callers; rate limiters are in-memory unless `REDIS_URL` is set; root `account_id==1` bypasses the tenant tie.

**Dangerous-sink entry points.** Outbound network (gated by `security.ValidateExternalURL` but redirect-following, SEC-044): `oidc.go`, `cas.go`, `oneroster_service.go`, `notification_delivery_service.go`. File IO: `internal/storage/{local,s3}.go`, `file_service.go`, download disposition in `handlers/files.go` (S3 bypass, SEC-018). Zip imports: `internal/qti`, `content_import.go` (decompression bombs, SEC-045/SEC-089). Raw SQL: gamification/quiz_submission/discussion repos — all verified parameterized (SEC-138). HTML sinks: `service.SanitizeHTML` across ~10 handlers (gaps at import paths and portfolio export). GraphQL: a single authenticated entry fanning to many unscoped/unauthorized resolvers (the concentrated liability, SEC-001/SEC-033/SEC-034/SEC-073).

### B. Dependency posture

- **Go:** `golang.org/x/net@v0.54.0` (5 html-parser advisories, fixed v0.55.0 — reachable via bluemonday but severity overstated; bump for hygiene) and `golang.org/x/crypto@v0.51.0` (14 SSH advisories, ssh subpackages not on any call path — SEC-126). Toolchain pinned to floating `golang:1.25`. **No `govulncheck` in CI** (SEC-125).
- **Frontend:** `npm audit` reports 0 vulnerabilities; the `overrides` block (undici/esbuild/rollup/braces) is applied. DOMPurify is at 3.4.3 vs 3.4.7 — keep the client sanitizer on the latest patch (SEC-142). `npm audit` is disabled in CI.
- **Containers:** base images pinned to floating tags, not digests; images run as root; no Trivy scan.
- **Supply chain:** all GitHub Actions pinned to mutable major tags (incl. the prod-SSH `appleboy/ssh-action@v1`); no Dependabot/Renovate. The manual npm `overrides` will silently go stale without automated updates.

---

*End of report. 102 confirmed security findings (5 Critical, 38 High, 24 Medium, 24 Low, 11 Info), 9 refuted/downgraded false positives, 10 manual follow-up areas, and ~70 code-organization findings across 8 dimensions. Critical and High findings carried a 3-lens adversarial verification; Medium/Low/Info received single-validator confirmation with no silent caps.*

---

## 9. Appendix B — Complete Verified Security Findings Register

_Auto-generated from the structured workflow output: all **164** raw findings that survived adversarial verification, before the cross-dimension consolidation applied in Section 3. Where the same sink was surfaced by multiple finder dimensions it appears more than once; those were merged into the ~102 distinct issues written up above. Sorted by severity._

| # | Severity | Finder dimension | Location | Finding |
|---|----------|------------------|----------|---------|
| 1 | CRITICAL | graphql | `internal/graphql/resolver.go:142 (resolveAllCourses) -> internal/service/course_service.go:76-78 (CourseService.List) -> internal/repository/postgres/course.go:51-58` | GraphQL allCourses returns every tenant's courses (cross-tenant enumeration) |
| 2 | CRITICAL | idor-objectref | `internal/api/v1/handlers/submissions.go:170-192` | Any enrolled student can read any classmate's assignment submission, score, grade, and attachments (GetSubmission) |
| 3 | CRITICAL | idor-objectref | `internal/api/v1/handlers/submissions.go:86-136` | ListCourseSubmissions returns every student's grades to any enrolled member when no user_id query param is supplied |
| 4 | CRITICAL | idor-objectref | `internal/api/v1/handlers/submissions.go:138-168` | ListSubmissions exposes all submissions for an assignment to any enrolled student, with no role check, no parent-tie, and no tenant scope |
| 5 | CRITICAL | authz-rbac | `internal/api/v1/handlers/sis_imports.go:141 (and 157,173,189); internal/service/sis_import_service.go:576,614,703` | Cross-tenant full-deployment PII export via SIS CSV export endpoints (account_id param discarded, query has no tenant filter) |
| 6 | HIGH | tenant-isolation | `internal/api/v1/handlers/quiz_questions.go:63,121,185 + internal/service/quiz_authoring_service.go:63-73` | Quiz question endpoints lack parent-tie + tenant scope (cross-tenant IDOR read/modify/delete) |
| 7 | HIGH | tenant-isolation | `internal/api/v1/handlers/quiz_question_groups.go:88,103,150` | Quiz question-group endpoints lack parent-tie + tenant scope (cross-tenant IDOR) |
| 8 | HIGH | tenant-isolation | `internal/service/commons_service.go:151-179 (buildSnapshot); handler internal/api/v1/handlers/commons.go:171-209` | Commons Publish snapshots arbitrary cross-tenant resources (data exfiltration) |
| 9 | HIGH | tenant-isolation | `internal/api/v1/handlers/courses.go ListCourses (scope=="all" branch) + internal/service/course_service.go:76-78` | GET /courses?scope=all returns all courses across all tenants to any authenticated user |
| 10 | HIGH | tenant-isolation | `internal/graphql/resolver.go:133-157 (resolveAllCourses)` | GraphQL allCourses query leaks all tenants' courses |
| 11 | HIGH | authz-rbac | `internal/api/v1/handlers/peer_reviews.go:77 (route: internal/api/v1/router.go:800)` | SubmitPeerReview lets any tenant user forge/overwrite any peer review (no reviewer-ownership check) |
| 12 | HIGH | authz-rbac | `internal/api/v1/handlers/announcements.go:170,240 (routes: internal/api/v1/router.go:661-662)` | Account-wide/global announcements can be edited or deleted by any authenticated user (in-handler authz skipped when CourseID is nil) |
| 13 | HIGH | authn-session | `internal/api/v1/handlers/users.go:604-644, internal/auth/jwt.go:60-77` | Account-admin can masquerade into a super_admin session, inheriting super_admin authority |
| 14 | HIGH | crypto-secrets-code | `internal/domain/models/oneroster_connection.go:11` | OneRoster connection ClientSecret stored plaintext despite "encrypted at rest" comment |
| 15 | HIGH | ssrf-outbound | `internal/service/notification_delivery_service.go:573 (also internal/auth/cas.go:31, internal/service/oneroster_service.go:53, internal/auth/oidc.go:271)` | Production SSRF-guarded HTTP clients follow redirects, allowing the guard to be bypassed with a 302 to an internal/metadata target |
| 16 | HIGH | graphql | `internal/graphql/resolver.go:428-444 (resolveCourseAssignments), 464-480 (resolveCourseModules); internal/repository/postgres/assignment.go:54-58; internal/repository/postgres/module.go:47-51` | Nested assignments and modules resolvers are not tenant-scoped — cross-tenant content leak via allCourses |
| 17 | HIGH | file-handling | `internal/api/v1/handlers/files.go:318-339 (route internal/api/v1/router.go:386; repo internal/repository/postgres/attachment.go:75-93)` | Cross-tenant file metadata enumeration via unscoped /folders/:folder_id/files |
| 18 | HIGH | file-handling | `internal/api/v1/handlers/files.go:287-302; internal/storage/s3.go:295-310` | S3 download path bypasses the F-040 Content-Disposition/nosniff inline-XSS control |
| 19 | HIGH | idor-objectref | `internal/api/v1/handlers/submissions.go:359-461` | ListSubmissionComments and CreateSubmissionComment let any enrolled student read/post on another student's submission |
| 20 | HIGH | idor-objectref | `internal/api/v1/handlers/developer_keys.go:43-64,99-141; internal/api/v1/handlers/auth_providers.go:76-113` | Cross-tenant IDOR: account-scoped admin endpoints trust the URL :account_id and never tie it to the caller's tenant |
| 21 | HIGH | dos-resource | `internal/qti/imscc.go:50-89 ; internal/service/imscc_parser.go:343-446,1423-1496` | IMSCC/QTI content import has no zip decompression-bomb guard (in-memory ReadAll of every entry) |
| 22 | HIGH | dos-resource | `internal/graphql/resolver.go:133-156,428-480,520-540` | GraphQL pagination perPage has no upper cap (unbounded SQL LIMIT / memory) |
| 23 | HIGH | headers-cors-csrf | `internal/auth/sso_handler.go:206-214 (also saml.go:737-745, oidc.go:229-233, handlers/mfa.go:301-309 & 368, handlers/passkeys.go:301-309 & 327-353)` | SSO / MFA / passkey / OIDC session cookies are set without the Secure flag |
| 24 | HIGH | saml-sso | `internal/auth/saml.go:583-630` | SAML assertions with no <Conditions> element bypass AudienceRestriction and expiry validation entirely |
| 25 | HIGH | saml-sso | `internal/auth/saml.go:447-455, 287, 328, 509-578` | No InResponseTo / AuthnRequest-ID binding: ACS accepts unsolicited responses and forged login (SAML login CSRF) |
| 26 | HIGH | business-logic | `internal/api/v1/handlers/coppa.go:197-249 (UpdateDPA), internal/service/coppa_service.go:182-198 (UpdateDPA/GetDPA), internal/repository/postgres/parental_consent.go:107-117 (FindByID/Update)` | Cross-tenant IDOR: any account-admin can read and tamper with another tenant's Data Processing Agreements |
| 27 | HIGH | business-logic | `internal/api/v1/handlers/coppa.go:120-133 (RevokeConsent), internal/service/coppa_service.go:121-136 (RevokeConsent), internal/repository/postgres/parental_consent.go:34-40 (FindByID)` | Cross-tenant IDOR: any account-admin can revoke another tenant's parental (COPPA) consent records |
| 28 | HIGH | frontend-auth | `web/src/pages/QuizStatisticsPage.jsx:215` | Stored XSS: quiz question_text rendered without sanitization on instructor/admin pages (QTI/IMSCC import bypasses server SanitizeHTML) |
| 29 | HIGH | tenant-isolation | `internal/api/v1/handlers/speedgrader.go:130 and internal/service/speedgrader_service.go:148` | SpeedGrader single-submission endpoint has no parent-tie or tenant scope (cross-tenant student grade/comment leak) |
| 30 | HIGH | tenant-isolation | `internal/api/v1/handlers/quiz_statistics.go:64 and internal/service/quiz_statistics_service.go:19` | Quiz statistics endpoint lacks quiz->course parent-tie and tenant scope (cross-tenant leak of all students' quiz answers/scores) |
| 31 | HIGH | tenant-isolation | `internal/api/v1/handlers/discussions_v2.go:190 and internal/service/discussion_v2_service.go:316` | Discussion entry V2 update has no ownership or parent-tie check (cross-tenant content tampering) |
| 32 | HIGH | tenant-isolation | `internal/api/v1/handlers/rubric_assessments.go:82 and internal/repository/postgres/rubric_assessment.go:23` | Rubric assessment Get/Update/List lack parent-tie and tenant scope (cross-tenant student-score read and grade tampering) |
| 33 | HIGH | authz-rbac | `internal/repository/postgres/student_accommodation.go:35 (FindByID), :47-48 (Delete), :51-55 (ListByUserID); internal/service/accommodation_service.go:67-83; internal/api/v1/handlers/accommodations.go:145-262` | Cross-tenant IDOR on student accommodations (disability/504/IEP data) — accommodation data layer has zero tenant scoping |
| 34 | HIGH | authz-rbac | `internal/api/v1/handlers/mastery_paths.go:137-163; internal/service/mastery_path_service.go:141-155` | Cross-course / cross-tenant IDOR: mastery-path ReplaceRule and DeleteRule never tie :rule_id to the URL course |
| 35 | HIGH | injection-xss | `internal/service/portfolio_service.go:525 (and :676)` | Stored XSS in portfolio HTML/PDF export — section Content injected unescaped |
| 36 | HIGH | injection-xss | `web/src/pages/ItemBankManagerPage.jsx:344 (also :407), web/src/pages/ItemAnalysisPage.jsx:125, web/src/pages/StimulusEditorPage.jsx:202` | Quiz item-bank/QTI question_text rendered with NO sanitization on three instructor pages |
| 37 | HIGH | graphql | `internal/graphql/resolver.go:119-131, 202-251, 428-480` | GraphQL has zero enrollment/role authorization: any in-tenant user reads any course's roster, assignments, and modules |
| 38 | HIGH | graphql | `internal/graphql/resolver.go:186-198, 315-353` | GraphQL user(id) leaks every in-tenant user's email and login_id to any authenticated user (selfOrAdmin bypass) |
| 39 | HIGH | idor-objectref | `internal/api/v1/handlers/speedgrader.go:129-171 (handler), internal/service/speedgrader_service.go:148-168 (service), internal/repository/postgres/submission.go:37-51 (repo)` | SpeedGrader GetStudentSubmission: cross-course / cross-tenant IDOR on any student's submission, grade, and grader comments |
| 40 | HIGH | idor-objectref | `internal/api/v1/handlers/rubric_assessments.go:76-147 (handler), internal/service/rubric_service.go:125-202 (service), internal/repository/postgres/rubric_assessment.go:23-29 + internal/repository/postgres/rubric_association.go:23 (repo)` | Rubric assessments: cross-course / cross-tenant IDOR to read, list, and overwrite any student's rubric grade and per-criterion data |
| 41 | HIGH | business-logic | `internal/api/v1/handlers/rubric_assessments.go:35-74 (CreateAssessment), :90-124 (UpdateAssessment); internal/repository/postgres/rubric_association.go:23-29 (FindByID)` | Rubric assessment Create/Update has no parent-tie: cross-course / cross-tenant grade write + gamification injection |
| 42 | HIGH | tenant-isolation | `internal/api/v1/handlers/enrollment_terms.go:110-210` | Cross-tenant IDOR: enrollment terms read/update/delete by ID with no tenant tie |
| 43 | HIGH | tenant-isolation | `internal/service/grading_period_service.go:32-42,63-76` | Cross-tenant IDOR: grading period groups & periods read/update/delete with hardcoded accountID=0 |
| 44 | HIGH | tenant-isolation | `internal/api/v1/handlers/feature_flags.go:39-90` | Cross-tenant config tampering: account-scoped feature flags trust :id with no tenant tie |
| 45 | HIGH | tenant-isolation | `internal/api/v1/handlers/outcome_proficiency.go:78-115` | Cross-tenant IDOR: account-scoped outcome proficiency scale read/overwrite/delete with no tenant tie |
| 46 | HIGH | authz-rbac | `internal/api/v1/handlers/enrollment_terms.go:35-127 (ListTerms, CreateTerm, GetTerm, UpdateTerm, DeleteTerm)` | Cross-tenant IDOR on enrollment-terms admin API (URL :account_id never tied to caller tenant) |
| 47 | HIGH | authz-rbac | `internal/api/v1/handlers/grading_periods.go:52-308; internal/service/grading_period_service.go:44` | Cross-tenant IDOR on grading-period-groups admin API (URL :account_id and group/period ids never tenant-scoped) |
| 48 | HIGH | authz-rbac | `internal/api/v1/handlers/calendar_events.go:89-128; internal/service/calendar_service.go:23` | Unauthenticated-context write: CalendarEvent.CreateEvent trusts attacker-supplied context_type/context_id with no authorization |
| 49 | HIGH | authz-rbac | `internal/api/v1/middleware/permissions.go:41-79 (RequireAdmin) cross-referenced with router.go account-scoped routes` | Account-scoped admin handlers are systematically tenant-blind: route-level RequireAdmin attests own-tenant admin only, handlers trust URL :account_id |
| 50 | HIGH | injection-xss | `web/src/pages/PortfolioPublicPage.jsx:752,763,619` | Stored XSS via javascript: href on the public (unauthenticated) portfolio page |
| 51 | HIGH | graphql | `internal/graphql/resolver.go:119` | GraphQL applies zero enrollment/role authorization on course(id) within a tenant — any student reads any course's roster, unpublished assignments, and modules |
| 52 | HIGH | idor-objectref | `internal/api/v1/handlers/calendar_events.go:89` | Cross-tenant / cross-course calendar event injection via POST /calendar_events |
| 53 | HIGH | idor-objectref | `internal/api/v1/handlers/discussion_checkpoints.go:154` | Discussion checkpoints: missing parent-tie + classmate progress IDOR (cross-course/cross-tenant tamper, same-course progress leak) |
| 54 | HIGH | idor-objectref | `internal/api/v1/handlers/course_paces.go:176` | Course paces: missing parent-tie; DeleteCoursePace has no tenant scope (cross-tenant destructive delete) |
| 55 | HIGH | idor-objectref | `internal/api/v1/handlers/audit.go:200` | Cross-tenant audit-log read: GET /accounts/:account_id/audit_log trusts the URL account id |
| 56 | HIGH | idor-objectref | `internal/api/v1/handlers/accommodations.go:172` | Accommodation write paths perform no in-handler authz and rely on a tenant-blind admin guard, enabling cross-tenant tamper of FERPA disability data |
| 57 | HIGH | business-logic | `internal/api/v1/handlers/submissions.go:533-561; internal/repository/postgres/submission.go:151-156; internal/api/v1/router.go:535-536` | PostGrades / HideGrades flip grade visibility for ANY assignment cross-course and cross-tenant (no parent-tie, no tenant scope) |
| 58 | MEDIUM | tenant-isolation | `internal/api/v1/handlers/sections.go:89-108 + internal/api/v1/handlers/authz.go:26-72` | GET /sections/:id leaks cross-tenant sections to admins (untenanted FindByID + isAdmin bypass) |
| 59 | MEDIUM | tenant-isolation | `internal/api/v1/middleware/permissions.go:133-137,172-175,250-268 + internal/api/v1/handlers/authz.go:26-72` | isAdmin role check bypasses course/tenant scope in course-role middleware and ResourceAuthorizer |
| 60 | MEDIUM | authz-rbac | `internal/api/v1/handlers/files.go:318 (route: internal/api/v1/router.go:386)` | ListFolderFiles exposes file metadata of any folder with no authorization or tenant scope (IDOR) |
| 61 | MEDIUM | authz-rbac | `internal/api/v1/handlers/document_annotations.go:267 / internal/service/document_annotation_service.go:143` | DeleteAnnotation authorizes against caller-supplied course_id instead of the annotation's real course (cross-course/cross-tenant delete) |
| 62 | MEDIUM | authn-session | `internal/auth/sso_handler.go:206, internal/auth/sso_handler.go:293, internal/auth/oidc.go:228, internal/auth/saml.go:737, internal/api/v1/handlers/mfa.go:301, internal/api/v1/handlers/mfa.go:368, internal/api/v1/handlers/passkeys.go:301` | Session cookie minted WITHOUT the Secure flag on every federated / MFA / passkey login path |
| 63 | MEDIUM | authn-session | `internal/auth/login_pipeline.go:347-358` | required_admin MFA policy does not force super_admin accounts to enroll or step up |
| 64 | MEDIUM | crypto-secrets-code | `internal/service/oauth2_service.go:128 (and internal/api/v1/handlers/oauth2.go:307)` | OAuth2 developer-key client secret stored in plaintext and compared with timing-unsafe != |
| 65 | MEDIUM | crypto-secrets-code | `internal/config/config.go:69-80` | JWT_SECRET length is never enforced — a 1-character production secret passes validation |
| 66 | MEDIUM | injection-xss | `internal/api/v1/handlers/submissions.go:222` | Student-submitted assignment Body persisted without server-side SanitizeHTML (rich-content control gap) |
| 67 | MEDIUM | injection-xss | `internal/api/v1/handlers/question_banks.go:127` | Legacy question-bank QuestionText accepted and stored without SanitizeHTML |
| 68 | MEDIUM | injection-xss | `web/src/pages/QuizStatisticsPage.jsx:215` | Frontend dangerouslySetInnerHTML sinks render question_text with no DOMPurify |
| 69 | MEDIUM | injection-xss | `internal/service/qti_import_service.go:180` | QTI / IMSCC / blueprint import paths copy QuestionText/Body verbatim without SanitizeHTML |
| 70 | MEDIUM | ssrf-outbound | `internal/auth/oidc.go:266` | OIDC discovery fetches token/authorization/JWKS sub-endpoints from the discovery document without re-validating them against the SSRF guard |
| 71 | MEDIUM | graphql | `internal/graphql/schema.go:79-103 (parseSelectionSet), internal/api/v1/handlers/graphql.go:20-61, internal/api/v1/router.go:628` | No query depth/complexity/field-repetition limit and no rate limit on /graphql (DoS amplification) |
| 72 | MEDIUM | graphql | `internal/graphql/resolver.go:186-198 (resolveUser), 119-131 (resolveCourse); compare router.go:223 and router.go:256` | GraphQL user(id) and course(id) bypass REST object-level authz (selfOrAdmin / enrolled) |
| 73 | MEDIUM | file-handling | `internal/api/v1/handlers/files.go:157-173 (route router.go:383)` | GetFile lacks course parent-tie: any enrolled user reads file metadata from sibling courses in the tenant |
| 74 | MEDIUM | idor-objectref | `internal/api/v1/handlers/document_annotations.go:98-130,396-424` | Document annotations on a submission are readable by any enrolled student (ListAnnotations / GetAnnotationSummary) |
| 75 | MEDIUM | idor-objectref | `internal/api/v1/handlers/custom_roles.go:53-75` | ListRoles trusts URL :account_id without a tenant tie (same class as dev-keys/auth-providers) |
| 76 | MEDIUM | dos-resource | `internal/api/v1/router.go:627-628` | /graphql endpoint has no rate limit |
| 77 | MEDIUM | dos-resource | `internal/api/v1/routes_public.go:38 ; internal/api/v1/handlers/oauth2.go:191-242` | OAuth2 token endpoint is not rate-limited (client_secret brute-force / request flood) |
| 78 | MEDIUM | dos-resource | `internal/api/v1/handlers/batch.go:197-228,230-260,145-194` | Bulk enrollment/message/date-shift endpoints accept unbounded input arrays |
| 79 | MEDIUM | saml-sso | `internal/auth/saml.go:719-751` | RelayState is used directly as the post-login redirect target with no allow-list/origin validation (open redirect) |
| 80 | MEDIUM | business-logic | `internal/api/v1/handlers/enrollments.go:70-108 (CreateEnrollment), internal/service/enrollment_service.go:55-76 (Create)` | CreateEnrollment does not verify the target user belongs to the caller's tenant |
| 81 | MEDIUM | frontend-auth | `web/src/components/RichContentViewer.jsx:17` | Client-side DOMPurify allowlist is far more permissive than the server (iframe data:/blob:, object, embed, style) — sole gate for import-only content |
| 82 | MEDIUM | tenant-isolation | `internal/api/v1/handlers/module_items.go:43 and internal/service/module_service.go:72` | Module item Get/Update/Delete/Move lack parent-tie and tenant scope (cross-tenant course-content tampering) |
| 83 | MEDIUM | tenant-isolation | `internal/api/v1/handlers/learning_outcomes.go:398` | Outcome results endpoint lets any enrolled student read a classmate's mastery results (?user_id=) |
| 84 | MEDIUM | tenant-isolation | `internal/api/v1/handlers/discussions_v2.go:162 and internal/service/discussion_v2_service.go:311` | Discussion entry V2 version-history read has no parent-tie or tenant scope (cross-tenant content/edit-history leak) |
| 85 | MEDIUM | authz-rbac | `internal/api/v1/handlers/portfolio.go:411-490 (UpdateSection/DeleteSection), 566-662 (UpdateArtifact/DeleteArtifact); internal/service/portfolio_service.go:173-241` | Same-tenant cross-portfolio IDOR: portfolio section/artifact write+delete skip the parent-tie that ReorderSections enforces |
| 86 | MEDIUM | authz-rbac | `internal/api/v1/middleware/permissions.go:200-237 (RequireSelfOrAdmin), 249-268 (isAdmin); internal/api/v1/handlers/authz.go:26-35 (ResourceAuthorizer.isAdmin)` | selfOrAdmin / RequireAdmin admin branch is tenant-blind on object-id routes, enabling cross-tenant admin access |
| 87 | MEDIUM | graphql | `internal/graphql/resolver.go:133-156, 520-541; internal/repository/postgres/course.go:61-62` | Negative perPage in GraphQL pagination becomes GORM Limit(-1) (no limit) — full-table dump bypassing the documented cap |
| 88 | MEDIUM | graphql | `internal/graphql/resolver.go:159-171, 428-444; internal/repository/postgres/assignment.go:54-58` | GraphQL exposes unpublished assignments (no workflow_state / enrolled gate) via assignment(id) and course.assignments |
| 89 | MEDIUM | file-handling | `internal/repository/postgres/attachment.go:23-45 (FindByID); internal/api/v1/handlers/files.go:175-203 (DeleteFile)` | Soft-deleted files remain fully downloadable and readable by ID (delete does not revoke access or purge bytes) |
| 90 | MEDIUM | idor-objectref | `internal/api/v1/handlers/calendar_events.go:75-87 (handler), internal/service/calendar_service.go:36-38 (service), internal/api/v1/router.go:485 (route)` | Calendar event GetEvent: any authenticated user can read any other user's private (User-context) calendar event by ID |
| 91 | MEDIUM | business-logic | `internal/api/v1/handlers/assignment_overrides.go:91-139 (CreateOverride); contrast :35-44 (overrideInCourse), :141-177 (GetOverride), :179-200 (UpdateOverride)` | Assignment override Create lacks the parent-tie enforced on Get/Update/Delete (cross-course / cross-tenant override write) |
| 92 | MEDIUM | business-logic | `internal/service/gamification/wiring/submission.go:55-129 (GradedSubmissionEmitCallback); internal/service/gamification/effects/award_currency.go:57-89; internal/repository/postgres/gamification_wallet.go:121-154` | Currency-award rules have no per-milestone idempotency; re-grading re-fires fresh events and re-awards currency |
| 93 | MEDIUM | tenant-isolation | `internal/api/v1/middleware/permissions.go:41-80` | Handler-layer gap: RequireAdmin authorizes the caller's role but never ties the :account_id path param to the caller's tenant |
| 94 | MEDIUM | authz-rbac | `internal/api/v1/handlers/feature_flags.go (SetAccountFeature / DeleteAccountFeature / GetAccountFeature / ListAccountFeatures)` | Cross-tenant write on account-scoped feature flags (SetAccountFeature/DeleteAccountFeature trust URL :id) |
| 95 | MEDIUM | authz-rbac | `internal/api/v1/handlers/outcome_proficiency.go (SetForAccount / GetForAccount / DeleteForAccount)` | Cross-tenant write/read on account-scoped outcome proficiency (SetForAccount/GetForAccount/DeleteForAccount trust URL :id) |
| 96 | MEDIUM | authz-rbac | `internal/api/v1/handlers/sis_imports.go (GetSISImport, GetSISImportErrors); CreateSISImport/ListSISImports also use raw :account_id` | Cross-tenant read of SIS import batches and errors (sub-resource handlers discard :account_id, fetch by batch id only) |
| 97 | MEDIUM | injection-xss | `web/src/pages/CollaborationsPage.jsx:241` | Stored XSS via javascript: href on collaboration documents |
| 98 | MEDIUM | injection-xss | `web/src/pages/ItemAnalysisPage.jsx:125; web/src/pages/StimulusEditorPage.jsx:202` | Two additional instructor pages render QTI-imported question_text raw (no DOMPurify) |
| 99 | MEDIUM | file-handling | `internal/api/v1/routes_quiz_extensions.go:50` | QTI/IMSCC import upload route has neither EnforceUploadSize nor a rate limiter, feeding an unbounded in-memory zip read (decompression-bomb DoS) |
| 100 | MEDIUM | file-handling | `internal/api/v1/handlers/files.go:211` | Download handler never checks attachment workflow_state, so soft-deleted files remain fully downloadable (confirmation with download-path evidence) |
| 101 | MEDIUM | idor-objectref | `internal/api/v1/handlers/conversations.go:201` | Cross-tenant conversation/inbox injection: CreateConversation accepts arbitrary recipient user IDs with no tenant or relationship check |
| 102 | MEDIUM | business-logic | `internal/api/v1/handlers/submissions.go:463-529; internal/api/v1/router.go:329` | BulkGrade does not tie each entry's assignment to the URL course (same-tenant cross-course grade write) |
| 103 | MEDIUM | business-logic | `internal/service/content_view_service.go:49-60; internal/service/gamification/wiring/content_view.go:39-138; internal/service/gamification/wiring/discussion.go:40-50; internal/service/gamification/effects/award_currency.go:58-89; internal/repository/postgres/gamification_wallet.go:71-168` | Gamification currency can be farmed by repeating student-controlled actions (page view, discussion post); idempotency only covers concurrent duplicates of one event |
| 104 | MEDIUM | business-logic | `internal/service/user_deletion_service.go:84-154; internal/domain/models/quiz_submission_answer.go:9` | FERPA deletion walk (EraseDependents) omits student-authored free-text PII (quiz answers, document annotations) |
| 105 | MEDIUM | business-logic | `internal/api/v1/handlers/peer_reviews.go:77-97; internal/service/peer_review_service.go:115-130; internal/api/v1/router.go:800` | Peer review submit has no reviewer-ownership check and accepts an unbounded score |
| 106 | LOW | authz-rbac | `internal/api/v1/router.go:665; internal/api/v1/routes_quiz_extensions.go:27,50-51` | Instructor/CourseRole guards on routes lacking a :course_id param silently fail closed (fragile guard wiring) |
| 107 | LOW | authn-session | `internal/api/v1/handlers/mfa.go:281-309, internal/api/v1/handlers/users.go:312-360` | Pending MFA / password-reset JWTs are not invalidated after a successful step-up |
| 108 | LOW | authn-session | `internal/auth/mfa_pending.go:64-82, internal/auth/password_reset_pending.go:69-87` | Pending JWTs omit iss/aud validation that the session path enforces |
| 109 | LOW | crypto-secrets-code | `internal/service/user_service.go:135-142` | Password-reset token stored in plaintext DB column (not hashed) |
| 110 | LOW | injection-xss | `internal/api/v1/handlers/conversations.go:347` | Conversation reply Body stored without SanitizeHTML (inconsistent with bulk-send path) |
| 111 | LOW | injection-xss | `internal/api/v1/handlers/submissions.go:398` | Submission comment text stored without SanitizeHTML |
| 112 | LOW | injection-xss | `web/src/components/RichContentViewer.jsx:17` | Frontend DOMPurify allowlist permits iframe/object/embed/form/style/data: for all rich content |
| 113 | LOW | ssrf-outbound | `internal/security/ssrf.go:51` | Promised SafeDialer never implemented — every guarded fetch is TOCTOU / DNS-rebinding vulnerable |
| 114 | LOW | ssrf-outbound | `internal/api/v1/handlers/notification_delivery.go:150` | Webhook communication-channel address accepted without SSRF/scheme validation at creation time |
| 115 | LOW | file-handling | `internal/api/v1/handlers/files.go:142; internal/service/file_service.go:119-137` | Upload Content-Type is attacker-controlled and stored unvalidated for unknown extensions |
| 116 | LOW | file-handling | `internal/api/v1/handlers/content_migrations.go:117-124` | content_migrations upload writes to a path built from the client filename |
| 117 | LOW | secrets-in-repo | `docker-compose.yml:26` | Dev docker-compose.yml ships a weak hardcoded JWT_SECRET, but production is guarded by a fatal default-value check |
| 118 | LOW | deps-vuln | `.github/workflows/ci.yml:9-102 (no govulncheck step)` | No govulncheck gate in CI — dependency/stdlib advisories are not caught at merge time |
| 119 | LOW | deps-vuln | `go.mod:27 (golang.org/x/crypto v0.51.0)` | golang.org/x/crypto@v0.51.0 has 14 open SSH advisories — upgrade for hygiene, not currently reachable |
| 120 | LOW | dos-resource | `internal/graphql/schema.go:79-137 ; internal/api/v1/handlers/graphql.go:20-61` | Hand-rolled GraphQL parser has unbounded recursion → unrecoverable stack-overflow crash |
| 121 | LOW | dos-resource | `internal/service/sis_import_service.go:122-228` | SIS import bcrypt-hashes a password for every CSV row with no row-count cap |
| 122 | LOW | headers-cors-csrf | `internal/api/v1/routes_public.go:28` | POST /logout has no CSRF protection (cross-site forced logout) |
| 123 | LOW | headers-cors-csrf | `cmd/server/main.go:1014 (fiber.New config) and internal/api/v1/middleware/security.go:55` | Trusted-proxy not configured; X-Forwarded-Proto is honored from any client for HSTS and protocol-derived logic |
| 124 | LOW | saml-sso | `internal/auth/saml.go:322-331, 578-648` | SubjectConfirmationData Recipient and NotOnOrAfter are parsed but never validated |
| 125 | LOW | saml-sso | `internal/auth/saml.go:884-906, 761-787` | Signature cert lookup is hardcoded to account_id=1 and uses first-parseable cert without matching assertion Issuer (multi-IdP cert confusion) |
| 126 | LOW | saml-sso | `internal/auth/saml_replay_cache.go:19-78` | Single-process in-memory replay cache leaves multi-pod SAML deployments replay-able (scope confirmation) |
| 127 | LOW | business-logic | `internal/service/coppa_service.go:45-86 (RequestParentalConsent), :88-118 (VerifyConsent)` | Parental-consent verification tokens never expire (ExpiresAt left nil on creation) |
| 128 | LOW | business-logic | `internal/service/submission_service.go:217-280 (Grade)` | Submission.Grade accepts NaN/Inf/unbounded values; no validation against PointsPossible or numeric sanity |
| 129 | LOW | business-logic | `internal/service/gamification/badge_service.go:217-235 (AwardToUser), internal/api/v1/handlers/gamification.go:706-738 (AwardBadgeToUser)` | Manual badge award does not validate the target user's tenant |
| 130 | LOW | business-logic | `internal/api/v1/handlers/assignments.go:242-249 (UpdateAssignment)` | Assignment publish toggle sets workflow_state directly, bypassing the state-machine used by Course |
| 131 | LOW | business-logic | `internal/service/quiz_attempts_service.go:305-347 (AnswerQuestion), internal/api/v1/handlers/quiz_submissions.go:176-229` | AnswerQuestion does not verify the question belongs to the submission's quiz |
| 132 | LOW | tenant-isolation | `internal/api/v1/handlers/wiki_page_revisions.go:59 and internal/repository/postgres/wiki_page_revision.go:23` | WikiPageRevision Get/Revert resolve by revision_id with no parent-tie or tenant scope (handler defined but currently unrouted) |
| 133 | LOW | authz-rbac | `internal/api/v1/handlers/ferpa.go:157-179` | FERPA GetExportRequest fetches export by id without tenant scope before the ownership check |
| 134 | LOW | injection-xss | `internal/api/v1/handlers/quiz_stimuli.go:82 and :116` | Quiz stimulus Content persisted without server-side SanitizeHTML on create and update |
| 135 | LOW | injection-xss | `internal/service/portfolio_service.go:633, :656, :659, :662, :559, :555` | Portfolio export allows javascript:/data: scheme URLs in href/src (escapeHTML does not block schemes) |
| 136 | LOW | graphql | `internal/graphql/resolver.go:58-72` | Root-field repetition is executed N times with no dedup, amplifying cost of expensive resolvers |
| 137 | LOW | file-handling | `internal/api/v1/handlers/content_import.go:66-69` | content_imports upload writes to a path built from the client-supplied filename (UUID prefix gives false safety) |
| 138 | LOW | file-handling | `cmd/server/main.go:1024; internal/api/v1/middleware/upload_size.go:32,38; internal/service/settings/catalog.go:306; internal/service/file_service.go:21` | Global Fiber BodyLimit (100 MB) contradicts the documented 5 GB safety net and silently caps the tunable per-tenant upload limit |
| 139 | LOW | file-handling | `internal/api/v1/router.go:508; internal/api/v1/handlers/content_migrations.go:118-124` | content_migrations file upload route is not behind EnforceUploadSize (only the global 100 MB BodyLimit and a rate limiter) |
| 140 | LOW | idor-objectref | `internal/api/v1/handlers/peer_reviews.go:43-59 (handler), internal/api/v1/router.go:798 (route)` | Peer reviews ListPeerReviews: missing assignment->course parent-tie allows same-tenant cross-course enumeration of reviewer/reviewee pairings and scores |
| 141 | LOW | idor-objectref | `internal/api/v1/handlers/discussion_entries.go:86-127 (handlers), internal/api/v1/router.go:374-375 (routes)` | Discussion entry UpdateEntry/DeleteEntry lack any ownership check (latent IDOR, currently masked by a route-param name mismatch) |
| 142 | LOW | injection-xss | `internal/api/v1/handlers/quiz_questions.go:109-111,166-172` | Quiz answer-feedback fields (correct/incorrect/neutral_comments) skip SanitizeHTML on create AND update (REST handler path) |
| 143 | LOW | injection-xss | `internal/storage/s3.go:302-309; internal/api/v1/handlers/files.go:296-302` | S3 file downloads bypass nosniff + Content-Disposition (presigned GET sets neither) |
| 144 | LOW | graphql | `internal/graphql/resolver.go:64` | GraphQL resolver errors return verbatim wrapped GORM/DB error text to the client (information disclosure) |
| 145 | LOW | file-handling | `internal/storage/local.go:22` | Local storage backend joins the storage key without confining the result to basePath |
| 146 | LOW | business-logic | `internal/service/rubric_service.go:169-217` | Rubric assessment score is summed from attacker-supplied per-criterion points with no validation against the rubric's defined points |
| 147 | LOW | business-logic | `internal/service/gamification/badge_service.go:217-235` | Manual badge award does not verify the target user belongs to the awarder's tenant |
| 148 | INFO | authz-rbac | `internal/api/v1/handlers/authz.go:26` | ResourceAuthorizer.isAdmin recognizes only role "admin", not "super_admin" (divergence from middleware) |
| 149 | INFO | injection-xss | `internal/api/v1/handlers/custom_gradebook_columns.go:208` | Custom gradebook column cell content stored without SanitizeHTML |
| 150 | INFO | secrets-in-repo | `.gitignore:12 (and whole tree)` | No tracked secrets, .env, or private keys in the repo or git history |
| 151 | INFO | secrets-in-repo | `cmd/seedtestdata/main.go:40; test/dex/config.yaml:38,44-50` | Dev seed/test fixtures contain shared static passwords (intentional, dev-only, not shipped in prod image) |
| 152 | INFO | secrets-in-repo | `internal/api/v1/handlers/mfa.go:107-134; internal/auth/*.go` | No secrets logged: TOTP/OIDC/LDAP secrets never written to slog/log/fmt; plaintext TOTP secret only returned to the enrolling user |
| 153 | INFO | deps-vuln | `web/package.json:36 (dompurify ^3.4.0, installed 3.4.3)` | Frontend prod dependencies several minors behind (DOMPurify 3.4.3 vs 3.4.7) — hygiene only, no open advisory |
| 154 | INFO | headers-cors-csrf | `internal/api/v1/middleware/csp.go:37 (mount site absent in cmd/server/main.go:1088-1103 and internal/api/v1/router.go)` | CSPNonce middleware (nonce + strict-dynamic CSP) is never mounted — documented hardening does not ship |
| 155 | INFO | headers-cors-csrf | `internal/api/v1/middleware/security.go:21` | style-src allows 'unsafe-inline' in the enforced production CSP |
| 156 | INFO | saml-sso | `internal/auth/saml.go:286, 509-578` | SAML Response Destination is never validated against the SP ACS URL |
| 157 | INFO | tenant-isolation | `internal/api/v1/router.go:374 and internal/api/v1/handlers/discussion_entries.go:87` | Discussion entry V1 Update/Delete routes use wrong path param (:id) vs handler (:entry_id) — fails closed but masks intended authz |
| 158 | INFO | authz-rbac | `internal/api/v1/handlers/conferences.go:133-336; internal/api/v1/handlers/collaborations.go:106-216` | Standalone conference/collaboration object routes default-allow when ContextType is not "Course" |
| 159 | INFO | injection-sql | `internal/repository/postgres/ (entire package), internal/service/, internal/graphql/, internal/db/schemagen/` | No SQL injection vectors found in the data-access layer (PASS 2 confirmation) |
| 160 | INFO | file-handling | `internal/api/v1/handlers/content_export.go:99-119` | Unwired ContentExportHandler.DownloadExport has an unsanitized :export_id used in a filesystem path |
| 161 | INFO | injection-sql | `internal/repository/postgres/ (entire package), internal/service/, internal/graphql/, internal/db/schemagen/, internal/storage/` | No SQL injection vectors in the data-access layer (Pass 3 confirmation) |
| 162 | INFO | graphql | `internal/graphql/resolver.go:505` | GraphQL ID arguments accept lax string-to-int parsing (Sscanf prefix match) allowing malformed/ambiguous IDs |
| 163 | INFO | file-handling | `internal/service/file_service.go:119` | Upload MIME validation trusts the client-supplied Content-Type header and never inspects file content (no magic-byte sniffing) |
| 164 | INFO | file-handling | `internal/service/file_service.go:236` | Trusted IMSCC import intentionally bypasses the SVG/HTML upload blocklist; safety rests entirely on download-time disposition + nosniff |

---

## 10. Appendix C — Complete Code-Organization Findings Register

_All **79** structural/organization findings (recommendations only; nothing was modified). Section 6 covers these thematically; this is the itemized list._

| # | Priority | Theme | Location | Finding | Recommendation |
|---|----------|-------|----------|---------|----------------|
| 1 | HIGH | god-files | `web/src/services/api.js` | web/src/services/api.js is a 2900-line god-module: ~460 flat CRUD methods on one object | Split by domain into web/src/services/api/<domain>.js (courses, assignments, quizzes, gradebook, discussions, portfolios, gamification, auth, admin/settings, ...), each importing the shared request/getCSRFToken/ApiError core from api/client.js. Re-export an aggregate api from api/index.js for back-compat, then migrate imports incrementally. Target ~15-20 files of 100-250 LOC each. |
| 2 | HIGH | god-files | `cmd/server/main.go:44` | cmd/server/main.go: 1121-line main() composition root (132 repos + 79 services + 88 handlers wired inline) | Extract a wiring package (or same-package files): buildRepositories(db) *Repositories, buildServices(repos, cfg) *Services, buildHandlers(svcs) *Handlers, plus registerEmitCallbacks(...) and runStartupBackfills(...). main() shrinks to orchestrating ~8 phase calls. Group the structs by domain bundle so related repos/services live together. |
| 3 | HIGH | god-files | `internal/api/v1/router.go:180` | Router.Register() is a 646-line method declaring ~333 routes in one function | Follow the existing registerSuperAdminRoutes precedent: extract one registerXxxRoutes(group) per section comment (registerQuizRoutes, registerGroupRoutes, registerGamificationRoutes, ...). Register() becomes ~30 helper calls. Keep the shared protected group + AuditWrites mount in Register, pass the group into each helper. |
| 4 | HIGH | god-files | `web/src/pages/GradebookPage.jsx:976` | GradebookPage.jsx: 1864-line file, 888-line GradebookPage component, 30 useState hooks | Move the four dialogs to web/src/pages/gradebook/dialogs/, the grid primitives (Cell/Header/Frozen* + GradebookGrid) to gradebook/grid/, and lift the 30 useState into a useGradebookState hook (or reducer) under gradebook/hooks/. GradebookPage becomes a thin composition shell. |
| 5 | HIGH | layering | `internal/api/v1/handlers/quizzes.go:133, sections.go:82, grading_standards.go:89, notification_delivery.go:192` | Repo-only handlers implement full CRUD + business rules with no service layer | Introduce/route through the existing service for each: quizzes.go should call into the quiz_service.go family; create thin QuizCrudService/SectionService/GradingStandardService wrappers that own SanitizeHTML + tenant scoping + the keyed Delete, and have the handler depend on *service.X instead of the repo. Target shape: handler -> service -> repo, never handler -> repo for writes. |
| 6 | HIGH | layering | `internal/api/v1/handlers/submissions.go:19-30, learning_outcomes.go:14-22, files.go:21-28, conversations.go:19-24, ai_assist.go:24-31, lti.go:84-101, users.go:35-48, document_annotations.go:15-24` | ~26 handlers inject repositories directly alongside services (mixed-layer dependency) | Split into two buckets. (a) Read-only cross-aggregate lookups used purely for tenant/authz checks (accountRepo.FindByID, enrollmentRepo.FindByUserAndCourse) are tolerable but better centralized in the ResourceAuthorizer (authz.go already does this for enrollment/user). (b) Any Create/Update/Delete a handler performs on a repo must move into the owning service. Make the rule explicit: handlers may hold *service.X only; repository.X in a handler constructor is a review flag. |
| 7 | HIGH | layering | `internal/service/enrollment_term_service.go:15,73; appointment_group_service.go:20,79,178; user_deletion_service.go:30,71,84,160; oneroster_service.go:30; sis_import_service.go:25; pairing_code_service.go:222` | Services hold raw *gorm.DB and run queries, bypassing the repository layer | Move the inline queries behind repository interfaces (EnrollmentTermRepository already exists as a concrete *postgres type — give the service the interface and add the Course-count query as a repo method). For multi-table transactions (user_deletion, appointment_group), introduce a repository method that accepts the unit-of-work, or a dedicated UnitOfWork abstraction, rather than a service holding *gorm.DB. Use typed workflow consts, not string literals. |
| 8 | HIGH | dead-dup | `internal/api/v1/handlers/planner.go, internal/api/v1/handlers/wiki_page_revisions.go, internal/api/v1/handlers/comment_bank.go` | Three complete feature stacks (Planner, Wiki Page Revisions, Comment Bank) are built but never mounted in the router | Decide per feature: if planned-soon, wire the routes now (and add a routing test so deadcode stops flagging them); otherwise delete the handler+repo+service+models+migration as a unit. The deadcode tool gives the exact symbol list. For comment_bank, either mount it or delete it plus its keyed-delete test. |
| 9 | HIGH | dead-dup | `internal/service/imscc_exporter.go, internal/service/imscc_export_metadata.go, internal/service/imscc_token_emitter.go` | IMSCC export stack (~1800 LOC) is dead — only the import side is wired | Either register ContentExportHandler.ExportCourse/DownloadExport in router.go (the handler is fully written) or remove the export stack and the roundtrip test. If export is a roadmap item, add a TODO at the router and a tracking issue so the intent is explicit rather than implied by orphaned code. |
| 10 | HIGH | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/service/` | internal/service is a 146-file flat package mixing ~25 domains; sub-group by bounded context | Split into domain subpackages mirroring the gamification/ precedent: internal/service/{assessment (quiz_*, question_bank, rubric, peer_review), grading (grading, gradebook, late_policy, grading_period, speedgrader, mastery_gradebook), content (page, module, wiki, assignment, discussion, announcement), migration (imscc_*, qti_*, content_migration, blueprint_*), identity (user, enrollment, auth_provider, oauth2, access_token, custom_role), rostering (sis_import, oneroster, enrollment_term), compliance (ferpa, coppa, accommodation, audit, user_deletion), communication (notification*, conversation, calendar, conference)}. Low intra-package coupling (see separate finding) makes this largely mechanical. |
| 11 | HIGH | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/api/v1/handlers/` | internal/api/v1/handlers is a 99-file flat package; the Router already groups it into ~25 domains via comments | Promote the router's comment sections into real subpackages: internal/api/v1/handlers/{assessment,grading,content,migration,identity,rostering,compliance,communication,gamification,...}. Because handlers are independent per-struct (no shared god-Handler), this is moveable file-by-file. Keep helpers.go/authz.go/canvas_aliases.go as a shared handlers/internal or handlers/common subpackage. |
| 12 | HIGH | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/api/v1/handlers/quizzes.go:15` | 20 of ~92 handlers bypass the service layer and call repositories directly (layering inconsistency) | Establish the invariant: handlers depend only on service interfaces, never on repository.*Repository. Route QuizHandler through quiz_service (which already exists) and migrate the other 19 bypassers. Add a CI lint (grep/staticcheck) that fails if internal/api/v1/handlers imports internal/repository for anything beyond shared types like PaginationParams. |
| 13 | HIGH | public-readiness | `internal/domain/models/account.go:36; setting.go:84; setup.go:202; ai_assist_service.go:9; migrations/000047_oidc_provider_columns.up.sql:11; registry.go:5` | Shipping source references gitignored CLAUDE.md (dangling docs in public repo) | Sweep tracked source for CLAUDE.md and [[...]] before release; repoint to PROJECT.md/CONTRIBUTING.md or inline the rationale. Add a CI grep gate. |
| 14 | HIGH | public-readiness | `docs/status/2026-05-15-phase-10-handoff.md` | Internal session-handoff doc ships, leaking local paths, PIDs, and the AI-agent workflow | Remove from the public repo (gitignore it like the sibling 2026-05-24-engineering-history-internal.md, already correctly excluded). |
| 15 | HIGH | testing | `.github/workflows/ci.yml:87-113` | Frontend vitest suite (35 test files) is never run in CI — pure test theater at the gate level | Add a frontend-test CI job (or a step in frontend-build) running npm run test with vitest in run mode, and make it a required check. Cheap, high-leverage: the tests already exist and pass locally — they just need to be wired to the gate. |
| 16 | HIGH | testing | `internal/repository/postgres/` | Repository/postgres layer: 120 implementations, only 2 test files — the tenant-isolation WHERE-clause core is largely untested | Adopt the existing PARITY_DB_URL integration harness (already wired in CI on pgvector/pgvector:pg16) and add table-driven cross-tenant tests for the highest-risk repos first: course-children (assignments, submissions, quizzes, discussions), polymorphic tables (files, rubric_assessments), and any repo with the accountID==0 escape hatch. One shared seedTenantAccount/seedTestUser helper file (the gamification wallet test already demonstrates the pattern) makes each repo test ~30 lines: seed two tenants, assert tenant B's id returns NotFound/empty. |
| 17 | HIGH | testing | `internal/service/` | Service layer: ~33 of ~95 services tested; high-risk auth/LTI/FERPA/COPPA/OAuth2 services have zero unit tests | Prioritize unit tests for ferpa_service, coppa_service, and the LTI/OAuth2 trio. These are pure-logic services that take repo mocks (66 testify mocks already exist) — no DB needed. Target the decision branches: FERPA disclosure-eligibility + deletion gating, COPPA messaging matrix (the 4 TestCreateConversation cases prove the matrix is testable), LTI claim/signature validation, OAuth2 grant validation. |
| 18 | HIGH | build-ci | `.github/workflows/ci.yml` | No SAST / dependency / container scanning anywhere in CI | Add a security workflow (PR + push to main, ideally also a weekly schedule:) with four cheap jobs: (1) govulncheck ./... (Go stdlib + module CVE scanner, fast, low-noise — make it blocking); (2) golangci/govulncheck-action or raw binary; (3) npm audit --audit-level=high in web/ (or osv-scanner) — start non-blocking, then promote; (4) Trivy fs scan + Trivy image scan of the two built images (wire into the existing docker job which already builds them). Consider GitHub CodeQL (free for this repo) for Go+JS as a fifth. Start non-blocking on the noisy ones, blocking on govulncheck, and document the gate in CLAUDE.md alongside the other CI gates. |
| 19 | HIGH | build-ci | `.github/workflows/ci.yml:27-31` | golangci-lint is advisory-only (continue-on-error: true) and pinned to a moving 'latest' | Add a checked-in .golangci.yml (enable at minimum errcheck, govet, staticcheck, ineffassign, unused, bodyclose, and ideally gosec as the SAST layer), pin the action to a fixed version: vX.Y.Z, and remove continue-on-error: true once the baseline is clean (use //nolint or config excludes for the known-noisy cases rather than disabling the whole gate). |
| 20 | HIGH | build-ci | `deployments/docker/Dockerfile.backend:18` | Production Docker images run as root (no USER directive in either Dockerfile) | Backend: add a non-root user in the final stage (RUN adduser -D -u 10001 app then USER 10001), ensure /data/files is owned/writable by it. Frontend: use the unprivileged nginx variant (nginxinc/nginx-unprivileged:alpine, serves on 8080) or add a non-root USER. In docker-compose.prod.yml add security_opt: ["no-new-privileges:true"], cap_drop: [ALL] (re-add only what nginx needs), and read_only: true with explicit tmpfs for the backend. Consider memory/cpu limits via deploy.resources. |
| 21 | HIGH | build-ci | `.github/workflows/ci.yml:87-113` | Frontend has 35 test files and an ESLint config, but neither runs in CI | Add npm run lint and npm run test steps to the frontend-build job (or a sibling frontend-test job). They reuse the same npm install cache, so cost is minimal. Make them blocking once green. |
| 22 | MEDIUM | god-files | `internal/service/imscc_parser.go` | internal/service/imscc_parser.go: 1983-line god-service (38 functions across every Canvas entity type) | Split into a package: imscc/manifest.go (parse/collect/flatten), imscc/import_<entity>.go (one per assignment/quiz/discussion/webcontent/weblink/questionbank), imscc/files.go (extractFiles/MIME/zip), imscc/html.go (body extract + rewriteAllBodies). Keep IMSCCParser as the orchestrator owning processOrganization/importResource. |
| 23 | MEDIUM | god-files | `internal/service/portfolio_service.go:515` | internal/service/portfolio_service.go mixes data CRUD with HTML/CSS/PDF rendering | Extract a portfolio_render.go (or internal/service/portfolio/render package) holding the HTML/CSS/PDF/README builders, with themes as Go templates or embed.FS .css assets rather than inline string literals. PortfolioService keeps only data orchestration and calls the renderer. |
| 24 | MEDIUM | god-files | `internal/api/v1/handlers/portfolio.go` | internal/api/v1/handlers/portfolio.go: 894-line god-handler (30 funcs spanning 7 sub-resources) | Split into portfolio_handler.go (portfolio CRUD + publish/public), portfolio_sections_handler.go, portfolio_artifacts_handler.go, portfolio_export_handler.go, and move the *ToJSON serializers to portfolio_dto.go. Keep them on the same PortfolioHandler receiver across files. |
| 25 | MEDIUM | god-files | `internal/service/imscc_exporter.go:312` | internal/service/imscc_exporter.go ExportCourse() is a 317-line function | Decompose ExportCourse into per-entity exportX helpers (exportAssignments, exportQuizzes, exportPages, exportFiles, writeManifest) and have ExportCourse orchestrate, matching the recommended import-side package split. |
| 26 | MEDIUM | god-files | `internal/auth/saml.go:509` | internal/auth/saml.go HandleACS() is a 244-line security-critical handler in a 979-line file | Extract validateSignedResponse (signature + single-assertion pin), extractAndValidateAttributes, and resolveOrProvisionUser as named, individually-testable steps; HandleACS orchestrates them in order. Do this under security review given the load-bearing checks. |
| 27 | MEDIUM | layering | `internal/service/enrollment_term_service.go:14; mastery_path_service.go:21,27; outcome_proficiency_service.go:15,18 (+ 14 service files import repository/postgres)` | Services depend on concrete *postgres.XxxRepository instead of the repository interface (DIP violation) | Define/΄use the matching repository.EnrollmentTermRepository / MasteryPathRepository / OutcomeProficiencyRepository interfaces and type the service fields on the interface. Wire the concrete *postgres type only at composition root (router/DI setup). Add a lint or test asserting no internal/service file imports internal/repository/postgres. |
| 28 | MEDIUM | layering | `internal/api/v1/handlers/audit.go:61-62,105-106; users.go:36-37; notification_delivery.go:15,21; lti.go:90,101` | Handlers consume persistence-layer types directly (postgres.AuditLogFilter / postgres.*Repository) at the HTTP boundary | Promote these to repository-package interfaces (AgeVerificationRepository, ParentalConsentRepository, CommunicationChannelRepository should live in internal/repository, not internal/repository/postgres) and move filter structs to the repository interface package or a service DTO. The handler should build a service/DTO filter that the service translates to the storage filter. |
| 29 | MEDIUM | dead-dup | `internal/service/discussion_v2_service.go` | DiscussionV2Service has 10 unused CRUD methods that duplicate the live DiscussionService (V1) | Delete the 10 unused V2 CRUD methods; keep only the read-state/threading methods that V2 actually adds. The handler already proves which 9 to keep. |
| 30 | MEDIUM | dead-dup | `internal/service/ferpa_service.go:499` | FERPAService.LogPIIAccess duplicates AuditService.LogPIIAccess; the FERPA copy is dead | Delete FERPAService.LogPIIAccess and route any future FERPA-side need through AuditService. Audit the other dead FERPAService methods (ProcessDeletion vs the wired ApproveDeletionRequest path) and remove the ones superseded by the live handler flow. |
| 31 | MEDIUM | dead-dup | `internal/domain/models/state_transitions.go` | workflow_state transition validators built for 4 entities, only Course wired (3 dead) | Either apply the validators in the Assignment/Submission/DiscussionTopic update handlers (the intended next wave) or trim to Course-only until those waves land. Track the remaining-3 application as an explicit issue so the gap between 'foundation exists' and 'foundation enforced' is visible. |
| 32 | MEDIUM | naming | `internal/repository/postgres/enrollment.go:12 and :16` | Repository struct types are named Repo but constructors are named Repository — a layer-wide constructor/struct mismatch | Pick one suffix for the concrete struct and apply it everywhere. Lowest-churn target: keep the constructor NewXxxRepository and the interface XxxRepository, and standardize the unexported struct on xxxRepo (the 133-file majority). Then rename the 4 stragglers (EnrollmentTermRepository, MasteryPathRepository, OutcomeAlignmentRepository, OutcomeProficiencyRepository structs) to xxxRepo for uniformity. Document the trio (interface=XxxRepository, struct=xxxRepo, constructor=NewXxxRepository) in CONTRIBUTING.md. |
| 33 | MEDIUM | naming | `internal/repository/postgres/grade_change_log.go (GradeChangeLogRepo) vs internal/repository/postgres/enrollment.go (enrollmentRepo)` | 22 repository structs are exported (capitalized) while 118 are unexported — inconsistent encapsulation | Standardize concrete repo structs as unexported (xxxRepo) — they are implementation detail behind the interface. Lower-case the 22 exported structs and confirm no external package references them (the gamification cluster is the largest sub-group, so check internal/service/gamification first). Add a lint/CI note alongside the existing TestGORMAcronymFieldsHaveColumnTag-style structural tests. |
| 34 | MEDIUM | naming | `internal/repository/postgres/outcome_alignment_repo.go:13, internal/repository/postgres/quiz.go, internal/repository/postgres/gamification_*.go` | 29 repository constructors return the concrete *XxxRepo pointer instead of the repository.XxxRepository interface | Define the missing XxxRepository interfaces in the repository package (in the matching *_interfaces.go file) and change these 29 constructors to return the interface, matching the ~90 that already do. Where an interface genuinely isn't warranted, document the exception in a code comment as is done elsewhere. |
| 35 | MEDIUM | naming | `internal/repository/postgres/student_accommodation.go, announcement.go, attendance.go, communication_channel.go, custom_gradebook_column.go, feature_flag.go, ferpa_compliance.go, notification_delivery.go, parental_consent.go, pairing_code.go, announcement_read_receipt.go` | 11 repository interfaces are declared inside the postgres adapter package instead of the central repository interfaces package | Move these 18 interface declarations into the repository package (matching *_interfaces.go files) and have the adapter constructors return the repository.-qualified type, consistent with the majority. This is a mechanical move with no behavior change. |
| 36 | MEDIUM | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/repository/postgres/feature_flag.go` | Repository interfaces are defined in two places: 30 in repository/*_interfaces.go but 11 still inside postgres/ adapter files | Move the 11 interface declarations from internal/repository/postgres/*.go up into internal/repository/<aggregate>_interfaces.go to match the documented convention. Add a small test/lint asserting no Repository interface type is declared under internal/repository/postgres. |
| 37 | MEDIUM | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/service/imscc_parser.go` | imscc/qti import-export logic (8 service files, ~5000 LOC) duplicates the domain of the dedicated internal/qti package | Consolidate all pure parse/serialize/token-rewrite logic into internal/qti (or a sibling internal/imscc), leaving only thin *_import_service.go / *_export_service.go orchestrators in the service layer (the pattern qti_import_service.go already follows). This both fixes the misplacement and removes ~4000 LOC of the largest files from the flat service package. |
| 38 | MEDIUM | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/cmd/server/main.go` | cmd/server/main.go is a 1176-line wiring monolith assembling 132 repos + 78 services + 88 handlers | Introduce per-domain wiring/provider functions colocated with each future service subpackage (mirroring internal/service/gamification/wiring/), e.g. assessment.Wire(db, deps) returning that domain's repos+services+handlers. main.go then composes ~10 domain wirings instead of 298 individual New* calls. The gamification 'wiring' subpackage is the existing precedent to generalize. |
| 39 | MEDIUM | public-readiness | `cmd/server/main.go:1110` | Local absolute path to a private Claude plan file embedded in shipping Go source | Drop the path; keep the behavioral note. Cover with the CI grep gate (block /Users/ and .claude/ in tracked source). |
| 40 | MEDIUM | public-readiness | `internal/service/imscc_roundtrip_test.go:66` | Hardcoded personal local path in a test fixture lookup | Delete the hardcoded absolute guess; rely on ROUNDTRIP_CARTRIDGE env plus the relative path (the test already skips when absent). |
| 41 | MEDIUM | public-readiness | `docs/research/gamification-2026-05/ (01-claude-wp-stack.md, 02-claude-lms-native.md, 03-claude-behavioral.md, 05-parallel-ai-prd.md, 06-claude-bootdev.md, SYNTHESIS.md, PHASE6-WAVE1-PLAN.md)` | Internal AI-research/PRD stream docs ship under docs/research/ | Move docs/research/gamification-2026-05/ out of the public repo (gitignore). If design rationale is worth publishing, write a single clean ADR instead. |
| 42 | MEDIUM | public-readiness | `docs/audits/2026-05-15-gamification-audit.md` | Public SECURITY.md links to an audit doc leaking internal branch state and CLAUDE.md refs | Sanitize the audit doc (drop branch/working-tree framing and /tmp paths, repoint CLAUDE.md refs), or de-link it from SECURITY.md. |
| 43 | MEDIUM | testing | `internal/service/gamification/wallet_service.go` | Gamification points/wallet core: integrity is tested, but the WalletService API surface and the dispatcher cooldown→award→ledger chain lack direct tests | Add wallet_service_test.go using the existing wallet repo mock: verify opt-out preference round-trips and that balance reads are user-scoped. Add a dispatcher_test.go that drives a seeded rule end-to-end and asserts exactly-one ledger row per (event,rule), covering the cooldown-then-award ordering. |
| 44 | MEDIUM | testing | `.github/workflows/ci.yml:27-31,62-69` | No coverage threshold gate and golangci-lint is continue-on-error — quality signals are advisory, not enforced | Add a coverage-diff gate (e.g. fail if patch coverage on changed Go files < N%, via codecov/coverage-diff or a small script over coverage.out). Drop continue-on-error on golangci-lint once the backlog is triaged, or at minimum make it block on new findings. |
| 45 | MEDIUM | build-ci | `deployments/docker/Dockerfile.frontend:6-7` | Frontend Dockerfile uses npm install (mutates lockfile) instead of npm ci | Change Dockerfile.frontend to COPY web/package.json web/package-lock.json ./ then RUN npm ci. Align CI to npm ci --legacy-peer-deps. npm ci requires the lockfile to exist and be in sync, which is the desired invariant. |
| 46 | MEDIUM | build-ci | `Dockerfile.backend:11` | No .dockerignore — full repo (including secrets, build artifacts, .claude worktrees) sent to Docker build context | Add a .dockerignore excluding at minimum: .git, .env, *.env, .claude/, web/node_modules, web/dist, bin/, prebuilt root binaries (paper-lms, server, seedtestdata, encrypt-ldap-passwords, paper-lms-server, leaderboard-snapshot), *.md, docs/, test/. This shrinks context dramatically and keeps secrets out of build layers. |
| 47 | MEDIUM | build-ci | `.github/workflows/ci.yml:18` | All GitHub Actions pinned to mutable major tags, not commit SHAs | Pin all actions to full commit SHAs with a trailing version comment (e.g. appleboy/ssh-action@<sha> # v1.2.0). At absolute minimum pin the appleboy/ssh-action and golangci/golangci-lint-action to SHAs. Add Dependabot's github-actions ecosystem to keep the SHAs updated (see the missing-Dependabot finding). |
| 48 | MEDIUM | build-ci | `.github/` | No automated dependency updates (no Dependabot / Renovate) | Add .github/dependabot.yml covering gomod (root), npm (web/), github-actions (auto-bumps the SHA pins), and docker (deployments/docker). Group patch/minor updates to reduce PR noise; schedule weekly. This pairs with govulncheck/Trivy: the scanner flags, Dependabot fixes. |
| 49 | MEDIUM | build-ci | `.github/workflows/ci.yml:140-179` | Production deploy is fully automatic on push to main with no human approval / environment gate | Wrap the deploy in a GitHub environment: production with required reviewers (even a single self-approval adds an audit trail and a deliberate gate). Add frontend-build (and, once blocking, the new security job) to the needs: list so a broken frontend or a high-severity CVE blocks the deploy. Keep the [skip deploy] escape hatch. |
| 50 | LOW | god-files | `internal/api/v1/handlers/gamification.go` | internal/api/v1/handlers/gamification.go: one handler spans currencies, badges, wallet, prefs, and vocabulary | Split into gamification_currency_handler.go, gamification_badge_handler.go, and gamification_wallet_handler.go on a shared receiver; keep resolveScope/derefBool in a small gamification_common.go. |
| 51 | LOW | god-files | `web/src/pages/PortfolioEditorPage.jsx` | Several 1000+ LOC JSX pages exceed the 800-LOC threshold with heavy local state | Prioritize RichContentEditor (shared) and PortfolioEditorPage: extract sub-components and collapse related useState into useReducer or custom hooks. Treat the rest as opportunistic refactors when next touched; migrate toward the documented <Page query={}> + useQuery pattern. |
| 52 | LOW | god-files | `internal/api/v1/handlers/users.go` | internal/api/v1/handlers/users.go: auth, profile, masquerade, password-reset, and COPPA fused in one 819-line handler | Split into auth_handler.go (Login/Register/Logout/SetPassword/cookies/password-reset), user_handler.go (GetUser/Update/profile/role/list), and masquerade_handler.go on the shared receiver. COPPA deps move with the auth/registration flows. |
| 53 | LOW | layering | `internal/api/v1/handlers/passkeys.go:212,233; mfa.go; setup.go:261` | GORM ORM sentinel (gorm.ErrRecordNotFound) leaks into the HTTP handler layer | Have repositories translate gorm.ErrRecordNotFound into a package-level repository.ErrNotFound (or return (nil,nil)) and let handlers check that sentinel. Keep gorm.io/gorm out of internal/api entirely. |
| 54 | LOW | layering | `internal/api/v1/handlers/health.go:15,21; setup.go:29,53` | Infra handles (*gorm.DB) injected into HTTP handlers for setup/health | Leave health.go as-is (or expose a tiny HealthChecker interface from db). For setup.go, push the bootstrap orchestration into a SetupService that owns the db/repos, leaving the handler to parse the request, check SETUP_BOOTSTRAP_TOKEN, and call the service. |
| 55 | LOW | dead-dup | `internal/service/gamification/mastery/mastery.go:78` | Mastery Calculator abstraction is unconstructed Wave-1 scaffolding; Method() accessor is dead on all 6 implementations | Leave as-is if the gamification mastery wave is imminent, but add a single registry/selector function (map[Method]Calculator) and reference it from the wiring layer so the package has at least one real entry point — that both validates the abstraction and lets deadcode prune anything genuinely surplus. Otherwise gate the package behind a build tag until wave 2. |
| 56 | LOW | dead-dup | `internal/service/grading_service.go:298 (and ~40 others)` | Assorted orphaned utility/service functions with zero callers (no entrypoint, no test) | Run deadcode -test ./cmd/... ./internal/... in CI (advisory, not blocking) and triage in batches. Prioritize the auth/audit cluster (confirm AuthAudit.MFAVerified/MFAFailed and CSPNonce are not SUPPOSED to be wired) then delete the rest. Provide list is reproducible from the tool. |
| 57 | LOW | dead-dup | `.claude/worktrees/` | 59 agent worktrees (3.6 GB) under .claude/worktrees are stale full-repo copies that pollute tooling | Prune merged/abandoned worktrees (git worktree prune plus removing the dirs) as part of the post-wave cleanup the CLAUDE.md already prescribes. Consider a periodic sweep so the count doesn't grow unbounded. |
| 58 | LOW | dead-dup | `internal/api/v1/handlers/helpers.go` | Mixed param-ID parsing idioms across handlers (c.ParamsInt vs strconv.ParseUint vs strconv.Atoi) | Add a single helper to handlers/helpers.go, e.g. func paramUint(c, name) (uint, bool) that returns false (and writes 400) on parse error, and migrate the 46 ParseUint/Atoi sites to it. Co-locate callerAccountID/callerUserID there too so the per-request extraction helpers live in one place. |
| 59 | LOW | naming | `internal/service/ferpa_service.go (FERPAService) vs internal/domain/models/*.go (FerpaClassification, FerpaViolation, GamificationFerpaFieldTag)` | FERPA acronym casing is split: all-caps FERPAService/FERPAHandler vs mixed-case Ferpa* in domain models and gamification | Decide one casing for FERPA (all-caps matches LTI/SIS/OIDC/COPPA precedent and the documented acronym convention) and align the domain-model + gamification types. If renaming model fields to FERPA..., add the required gorm:"column:ferpa_..." tags in the same change so AutoMigrate column names don't drift, and update TestGORMAcronymFieldsHaveColumnTag expectations. |
| 60 | LOW | naming | `internal/repository/postgres/outcome_alignment_repo.go` | One repository file uses the _repo.go filename suffix; all 119 peers use the bare-entity convention | Rename to internal/repository/postgres/outcome_alignment.go to match the bare-entity convention. If the name would collide with quiz_outcome_alignment.go semantics, prefer a clearer entity name over a layer suffix. |
| 61 | LOW | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/domain/models/` | domain/models is a 121-file flat package, but here flat is correct — keep it as the shared vocabulary, only separate the logic files | Leave the data structs flat in domain/models. Optionally move the state machine + enums into internal/domain/workflow (or a models/workflow subpackage that imports models) so the data layer stays declarative and the transition rules have an obvious home. Record in PROJECT.md that models is intentionally flat to avoid shared-type cycles. |
| 62 | LOW | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/api/v1/handlers/courses.go` | handlers depend on concrete *service.XxxService types, not interfaces — couples HTTP layer to service implementations | When splitting service into domain subpackages, define the handler-facing interface in (or near) each handler subpackage and have the handler depend on that, so a handler subpackage imports an interface it owns rather than a foreign concrete type. This both decouples and prevents handler->service import fan-out across the new subpackages. |
| 63 | LOW | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/repository/postgres/` | internal/repository/postgres is a 120-file flat package, but it correctly mirrors the (also-flat) repository contracts — defer splitting until the contracts split | Lower priority than service/handlers. If/when the repository contract package is grouped by domain, mirror that grouping in postgres/ (internal/repository/postgres/{assessment,grading,...}) for symmetry. Until then, leave it. First fix the 11 misplaced interfaces (separate finding). |
| 64 | LOW | public-readiness | `README.md:43` | README quickstart points users to wrong port (localhost:8080) | Update the README to the actual published port (http://localhost), or add an 8080:80 mapping if 8080 is intended. Align README, PROJECT.md, and the compose file. |
| 65 | LOW | public-readiness | `STALE_COLUMNS.md` | STALE_COLUMNS.md ships a generated schema-tooling report to the public repo root | Move under docs/ or gitignore it (regenerate via make stale-cols). If kept, label it a generated CI artifact. |
| 66 | LOW | testing | `internal/testutil/mocks/` | Mock organization is exemplary (66 per-entity testify mocks) but several mocks appear orphaned, signaling untested consumers | Use the orphaned-mock set as the prioritized backlog for service unit tests (conversation_service is COPPA-adjacent and should jump the queue). Keep the per-entity testify convention; do not introduce a second mock framework. |
| 67 | LOW | testing | `internal/` | Table-driven style is inconsistent — only ~26 of 149 backend test files use it; auth tests are the good model | For matrix-shaped concerns (role × tenant × resource), prefer one table-driven test over N functions. Adopt the auth/mastery test files as the house style in CONTRIBUTING. Not a rewrite — apply going forward and when touching a handler/service. |
| 68 | LOW | testing | `internal/graphql/, internal/storage/` | GraphQL and storage packages have zero tests despite being externally reachable surfaces | Add a resolver test asserting tenant scoping flows from graphql/context.go, and a storage test for the local backend's key/path handling (the S3 path can stay integration-gated). Small files; an afternoon closes both. |
| 69 | LOW | testing | `web/src/` | Frontend test coverage is sparse and concentrated — ~24 of 197 pages+components tested, apiQueries data layer untested | After wiring vitest into CI (Finding 1), add tests for apiQueries.js (mock the api layer; assert query keys, error propagation, and tenant-scoped params) since it is the shared dependency. Don't chase per-page coverage on legacy pages; cover shared services/contexts/hooks. |
| 70 | LOW | build-ci | `deployments/docker/Dockerfile.frontend:2` | Node version drift: CI builds on Node 22, production image builds on Node 20 | Pin both to the same Node major (bump Dockerfile.frontend to node:22-alpine, or pin CI to 22 — pick one and keep them in lockstep, ideally referenced from a single source like an .nvmrc). |
| 71 | LOW | build-ci | `deployments/docker/Dockerfile.backend:2` | Docker base images pinned only to floating minor tags, not digests | Pin base images by digest (golang:1.25-alpine@sha256:..., alpine:3.20@sha256:..., nginx:1.27-alpine@sha256:..., postgres:16.x-alpine@sha256:...) and let Dependabot's docker ecosystem bump the digests. At minimum pin nginx:alpine to a specific minor. |
| 72 | LOW | build-ci | `.github/workflows/ci.yml:24-25` | go vet runs in the (non-blocking) lint job, not the test job — and is the only blocking-capable static check, yet shares a job that can be skipped | Move go vet ./... into the backend-test job (which is already in the deploy needs:), or add backend-lint to the deploy needs:. Document in CLAUDE.md which jobs are actually required via branch protection so the 'gates block merge' claim is verifiable. |
| 73 | INFO | naming | `internal/api/v1/handlers/ (assignments.go vs gamification.go)` | Handler filenames mix collection-plural and feature-singular forms without a documented rule | No bulk rename needed. Document the implicit rule in CONTRIBUTING.md: 'CRUD-on-an-entity handlers are pluralized after the entity; cross-cutting feature/subsystem handlers are singular after the feature.' That converts ad-hoc choices into a citable convention. |
| 74 | INFO | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/service/` | Intra-service coupling is very low (14 sibling-service field refs), so the split is safe and low-risk | Treat as supporting evidence for the service-split finding. Where a cross-domain edge would invert (rare), introduce a small interface in the consuming package rather than importing the concrete type. No action needed beyond using this to de-risk the refactor. |
| 75 | INFO | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/security/ssrf.go` | Small single-file top-level packages (obs, config, security, settingsctx) are healthy and should be the model — note the security/ssrf split from service | No change. Use auth/, storage/, security/, and service/gamification/ as the in-repo reference patterns when sub-grouping the three oversized packages so the new layout feels native. |
| 76 | INFO | package-boundaries | `/Users/alfred/Projects/Paper LMS/paper-LMS/internal/` | Proposed target tree: domain-grouped service/handlers, lifted repo interfaces, consolidated content-interchange package | Sequence: (1) lift the 11 misplaced repo interfaces (cheap, no behavior change); (2) route the 20 bypass handlers through services and add the no-repo-import lint; (3) consolidate imscc/qti parse logic into internal/qti; (4) extract per-domain wiring funcs to shrink main.go; (5) finally move service + handler files into domain subpackages one domain at a time (start with assessment or interchange — highest file count, lowest external coupling). Validate each step against the existing schema-parity and acronym CI gates. |
| 77 | INFO | public-readiness | `docs/super-admin.md:3; docs/state-dpa/multi-pod-verification.md:1` | docs/super-admin.md and docs/state-dpa carry unexplained internal Wave/Phase vocabulary | Translate internal Wave/Phase status banners into plain capability statements for public docs. |
| 78 | INFO | public-readiness | `.gitignore / .env.example / LICENSE / SECURITY.md / README.md` | Healthy: gitignore, .env.example, license/meta files, TODO density, debug prints | No action needed here. Focus remediation on the internal-artifact leaks above. |
| 79 | INFO | testing | `internal/api/v1/handlers/` | Handler test coverage is the project's strongest layer — tenant-isolation patterns are well exercised | Keep this as the canonical pattern. Backfill the untested handlers that touch sensitive data next: enrollments, gradebook, files (download Content-Disposition allow-list), oauth2, external_tools, access_tokens, ferpa — none currently have a _test.go. |

---

## 11. Appendix D — Manual Follow-Up Gaps (Completeness Critic)

| # | Area | Why it needs a human pass | Suggested search |
|---|------|---------------------------|------------------|
| 1 | LTI AGS line-item grade write/read — untenanted, no parent-tie (PostScore, GetResults, UpdateLineItem, GetLineItem) | This entire LTI subsystem (~1,350 LOC) is essentially unreviewed in the confirmed findings, which only touch DeleteLineItem-style routes and standard REST grade endpoints. internal/repository/postgres/lti_line_item.go:23 FindByID(ctx, id) takes NO accountID — only Delete got the F-012 widening (line 35-49). Consequently the service internal/service/lti_ags_service.go:101 PostScore and :192 GetResults resolve a line item with zero tenant scope, and the handlers internal/api/v1/handlers/lti.go:597 PostScore / :666 GetResults / :499 UpdateLineItem never verify lineItem.CourseID == URL :course_id (contrast DeleteLineItem at :561-578 which correctly checks both). PostScore also syncs into the gradebook (syncSubmissionScore, :161). Net: any instructor in any tenant can read or overwrite any student's LTI grade across tenants by guessing/iterating a line-item ID, and the userId in the score body is fully attacker-controlled. This is a distinct cross-tenant IDOR + grade-injection class the audit did not enumerate. | Read internal/api/v1/handlers/lti.go:443-688 (CreateLineItem/GetLineItem/UpdateLineItem/PostScore/GetResults) and internal/service/lti_ags_service.go fully; grep -n 'FindByID' internal/repository/postgres/lti_line_item.go and confirm only Delete is tenant-scoped |
| 2 | LTI NRPS roster export (GetMemberships) leaks full course roster PII with no tenant or enrollment-tie | internal/api/v1/handlers/lti.go:696 GetMemberships parses :course_id and calls internal/service/lti_nrps_service.go:30 GetMemberships(ctx, courseID, params) which calls enrollmentRepo.ListByCourseID(ctx, courseID, 0, params) — accountID hardcoded 0 (line 32). The route is guarded only by 'enrolled' (router.go:361), but 'enrolled' resolves enrollment for the URL course; combined with the untenanted repo call, the handler returns every member's user.id, name, email, and LTI roles for any course id without confirming the course belongs to the caller's tenant. NRPS is a bulk-PII export endpoint and is not in the confirmed findings. | Read internal/service/lti_nrps_service.go entirely; check whether RequireEnrolled (permissions.go) validates the course's account_id; grep -n 'ListByCourseID' internal/repository/postgres/enrollment.go for tenant scope |
| 3 | AI Assist endpoint is authenticated-but-unauthorized: any student can drive paid Anthropic API calls (financial DoS / cost amplification) and there is no server-side input-length cap before billing | internal/api/v1/routes_p3_features.go:95 mounts POST /ai_assist/:action in the generic 'protected' group with only AIAssistRateLimit() — no RequireInstructor/RequireAdmin. internal/api/v1/handlers/ai_assist.go:41 Dispatch checks only that user_id is set (the COPPA gate only blocks k5/m68/coppa_strict tenants). input.Text has no length validation before being sent to api.anthropic.com (ai_assist_service.go:233 doOnce). Any student in a standard tenant can flood the org's Anthropic key; the rate limiter is in-memory so multi-pod splits the budget. No confirmed finding covers AI Assist authz or its unbounded-input cost vector. | Read internal/api/v1/handlers/ai_assist.go:95-140 for role checks and input bounds; grep -n 'AIAssistRateLimit' internal/api/v1/middleware/ratelimit.go for the budget; check truncate() call sites in ai_assist_service.go |
| 4 | LTI 1.1 SharedSecret (and LTI tool ConsumerKey/private key) stored in plaintext DB columns — same class as the confirmed OAuth2/OneRoster plaintext-secret findings but a distinct, unlisted credential | internal/domain/models/context_external_tool.go:15 SharedSecret is a plain string with only json:"-" (hidden from API output, NOT encrypted at rest). It is set verbatim from the request body in external_tools.go CreateExternalTool/UpdateExternalTool (:132, :190) and never round-tripped through internal/auth/secretbox.go. The CLAUDE.md invariant says 'Any DB-resident secret round-trips through secretbox.Encrypt. No plaintext secret columns.' LTI tool credentials are a DB-resident secret and violate this. Also confirm whether LTI 1.3 tool-configuration private keys (lti_tool_configuration.go) are encrypted. | grep -rn 'SharedSecret\/secretbox' internal/service/external_tool_service.go internal/domain/models/context_external_tool.go internal/domain/models/lti_tool_configuration.go; check the migration for context_external_tools / lti_tool_configurations column types |
| 5 | Audit log (AuditWrites) attributes masquerading-admin writes to the victim user — non-repudiation failure compounding the confirmed masquerade-to-super_admin finding | internal/api/v1/middleware/audit_writes.go:47 records only c.Locals('user_id'), which during masquerade is the IMPERSONATED target, not the admin actor. The actor identity (masquerade_by / admin_account_id Locals, set in auth.go) is never captured. Given the confirmed [high] that an account-admin can masquerade into a super_admin session and inherit its authority, every destructive write they perform is logged under the victim's identity, defeating the FERPA/COPPA-relevant audit trail. The audit reviewed masquerade auth but not whether the immutable write-audit preserves the true actor. | Read internal/api/v1/middleware/audit_writes.go fully and the AuditService.Record signature in internal/service/audit_service.go; grep -n 'masquerade_by\/admin_account_id' internal/api/v1/middleware/ to see if any audit path threads the actor |
| 6 | Outbound webhook / external-tool delivery integrity — no HMAC signature, and SSRF redirect-follow already confirmed; receivers cannot authenticate Paper LMS as sender (spoofable webhooks) | internal/service/notification_delivery_service.go has no hmac/X-Signature/signing of the outbound webhook body (grep for hmac/Signature returned nothing relevant). Combined with the confirmed redirect-follow SSRF and creation-time scheme gap, webhook receivers have no way to verify payload authenticity/integrity, and a tenant can register a webhook that exfiltrates notification PII to an arbitrary endpoint. The audit covered the SSRF egress but not payload signing / replay protection on the delivery side, which is a standard webhook security control. | Read internal/service/notification_delivery_service.go around the HTTP POST (line ~560-600); grep -n 'hmac\/Signature\/X-Hub\/sign' internal/service/notification_delivery_service.go internal/api/v1/handlers/notification_delivery.go |
| 7 | Mass-assignment via repo .Save()/.Updates() of body-bound structs — privilege-field tampering (e.g., workflow_state, account_id, points, ownership FKs) on Update paths that BodyParser the full model | Many Update repos use db.Save(&model) (account.go:32, announcement.go:46, assignment_group.go:38, appointment_group.go:39, etc.) which writes EVERY column. The audit confirmed the assignment publish toggle bypasses the state machine but did not systematically check whether Update handlers re-bind attacker-controllable privilege fields (account_id, user_id/owner, points_possible, is_admin-adjacent, workflow_state) from the JSON body into the persisted struct. Save() of a body-populated struct is a classic mass-assignment sink. A focused pass over the ~30 Save() call sites mapped against which handler populates the struct from c.BodyParser is warranted. | For each handler Update method, check whether it BodyParses into the same struct it passes to repo.Update/Save without resetting AccountID/UserID/ID/workflow_state from the loaded row; grep -rn 'BodyParser' internal/api/v1/handlers/ then cross-ref repo .Save calls |
| 8 | WebAuthn/passkey registration & assertion verification (RP ID, origin, challenge binding, user-handle ownership) — passkey is an MFA-bypass-by-design path but its crypto verification was not reviewed | CLAUDE.md/architecture note passkey (ProviderType=='passkey') skips the MFA gate because 'the assertion IS the second factor' — so the assertion verification is the entire security boundary, yet no confirmed finding examines internal/auth/webauthn*.go for correct RP ID / origin allow-list, challenge single-use binding, sign-count regression detection (cloned-authenticator), or that the returned credential is bound to the authenticating user (user-handle confusion / account takeover). passkeys.go handlers (begin/finish) were only touched for the missing Secure cookie flag. | Read internal/auth/webauthn.go and handlers/passkeys.go begin/finish; grep -n 'RPID\/RPOrigin\/Origin\/Challenge\/SignCount\/UserHandle\/VerifyLogin\/VerifyRegistration' internal/auth/webauthn*.go |
| 9 | Scheduler / background jobs (leaderboard-snapshot, FERPA deletion, periodic tasks) iterate tenant data with accountID=0 escape hatch — verify they don't cross tenants or run unsanitized | internal/scheduler runs JobFuncs with no tenant context, and cmd/leaderboard-snapshot/main.go plus gamification rule jobs touch tenant rows. The observer/pairing-code services already thread accountID=0 pervasively (observer_service.go uses 0 in ~8 places). Background jobs are an off-the-request-path execution context where the (ctx,id,accountID) tenant discipline can silently degrade to 0. No confirmed finding examines scheduled-job tenant isolation or what data the leaderboard snapshot aggregates across accounts. | Read cmd/leaderboard-snapshot/main.go and internal/service/gamification/rule_service.go job entry points; grep -rn ', 0)' internal/service/observer_service.go internal/service/gamification/ to map accountID=0 usage in non-auth contexts |
| 10 | Pairing-code generation entropy/format and observer-link tenant isolation (FERPA-sensitive parent linkage) | internal/service/pairing_code_service.go:189 GeneratePairingCodeString and Redeem (:210) are the parent/observer linkage primitive — a weak/short/predictable code or missing rate-limit on Redeem allows an attacker to brute-force a pairing code and link themselves as observer to an arbitrary student (full grade/PII access). LinkObserverToStudent (observer_service.go:131) uses accountID=0 for both user lookups, so cross-tenant observer linkage may be possible if a code is guessed. Redeem has no visible attempt limiter. The audit did not assess code entropy, redeem brute-force, or cross-tenant linkage. | Read models.GeneratePairingCodeString (grep -rn 'GeneratePairingCodeString' internal/domain/models/); check Redeem rate-limiting in pairing_codes handler/route; verify LinkObserverToStudent enforces same-tenant for observer and student |

---

## 12. Appendix E — Independent Verification Addendum & Remediation Plan

_Added 2026-05-30 after the automated audit. The orchestrator re-read each cited code path by hand to confirm the agent findings are real (not hallucinated locations) before any remediation. Verdict: the Critical/High tier holds up — 5/5 Criticals confirmed real and accurately located, and the completeness critic surfaced a Critical the finders missed (LTI AGS)._

### E.1 Hand-verified verdicts

| ID | Finding | Route guard | Verdict | Notes |
|----|---------|-------------|---------|-------|
| SEC-001 | GraphQL `allCourses` cross-tenant enumeration | `Protected` only (no role) | **CONFIRMED** | `CourseService.List` → `courseRepo.List(ctx, 0, params)`; repo filters only `if accountID != 0`. |
| SEC-002 | `GetSubmission` reads any classmate's submission | `enrolled` | **CONFIRMED** | `submissions.go:181` no owner/teacher/observer check; PII audit logs but does not gate. `quiz_submissions.go:149` shows the correct guard. |
| SEC-003 | `ListCourseSubmissions` leaks all grades when `user_id` omitted | `enrolled` | **CONFIRMED** | `submissions.go:129` filter is opt-in (`if filterUserID > 0`); omit the param → no filtering. Intra-course (enrolled ties to tenant). |
| SEC-004 | `ListSubmissions` exposes all assignment submissions | `enrolled` | **CONFIRMED** | `submissions.go:146` `ListByAssignment` — no user filter, no role check, no parent-tie. |
| SEC-005 | SIS CSV export = full-deployment PII dump | `admin` (`RequireAdmin`) | **CONFIRMED (worse)** | `sis_imports.go:142/147` parses `account_id` then **discards it** (`_,`) and calls `ExportUsersCSV(ctx)` with no account arg. Every tenant's users/courses/sections/enrollments, even for a correctly-scoped admin. |
| Gap-1 | LTI AGS untenanted line items | `instructor`/`enrolled` | **CONFIRMED — elevate to CRITICAL** | `lti_line_item.go:23` `FindByID(ctx, id)` has no accountID. `PostScore` (`lti.go:597`) + `UpdateLineItem` (`:500`) + `GetResults` (`:666`) skip the parent-tie that `DeleteLineItem` (`:574`) performs → cross-tenant grade **read and write** with an attacker-controlled `userId`. |
| Gap-2 | LTI NRPS roster PII | `enrolled` | **CONFIRMED (High)** | `lti_nrps_service.go:32` `ListByCourseID(ctx, courseID, 0, …)`. `enrolled` blocks cross-tenant, but any enrolled **student** can pull the full roster's names/emails/roles. |
| Gap-3 | AI Assist no authz + no input cap | `Protected` + rate-limit only | **CONFIRMED (High)** | `routes_p3_features.go:95` no role gate; `ai_assist.go:73` only checks `text != ""` — unbounded input forwarded to a paid Anthropic API → cost amplification. COPPA gate only covers k5/m68/coppa_strict. |
| Gap-4 | LTI 1.1 `SharedSecret` plaintext at rest | n/a | **CONFIRMED (Med/High)** | `context_external_tool.go:15` `SharedSecret string` (`json:"-"` hides it from the API but it is not encrypted) — violates the secretbox "no plaintext secret columns" invariant. |

### E.2 Root cause

Two recurring patterns, not 164 unrelated bugs:

1. **Object reads resolve by primary key with no parent-tie and no tenant filter.** The `F-012/F-013` hardening (parent-tie + trailing-`accountID`) was applied to several *Delete* handlers (e.g. `DeleteLineItem`) but **not to the sibling read/update/score handlers**. Fix the read/update paths to the same standard.
2. **Service/repo methods that take no `accountID`, or hardcode `0`.** `CourseService.List(ctx, 0, …)`, `lti_line_item.FindByID(ctx, id)`, `lti_nrps … ListByCourseID(ctx, id, 0, …)`, `SISImportService.Export*CSV(ctx)`. The `accountID==0` escape hatch is leaking onto request-facing paths.

### E.3 Remediation plan

**P0 — fix before any public exposure (cross-tenant data + grade integrity):**
- [ ] **SEC-005** — thread the caller's tenant: `Export*CSV(ctx, accountID)`; in the handler use `callerAccountID(c)` and reject/cross-check the `:account_id` param via `assertSameTenant`. Add the tenant `WHERE` to the export queries (`sis_import_service.go:576/614/703`).
- [ ] **SEC-001** — add `accountID` to `CourseService.List`; pass `AccountIDFromContext(ctx)` from `resolveAllCourses` and `callerAccountID(c)` from the REST `scope=all` path; reject `accountID==0` on request-facing callers. Sweep every `resolveAll*` resolver for the same omission.
- [ ] **SEC-002/003/004** — in the three submission read handlers: load, then if `submission.UserID != callerUserID` require Teacher/TA (from `enrollment_type` Locals) or a verified observer link, else `responses.NotFound`; make the `ListCourseSubmissions` filter default-deny (students without an explicit self/observee scope get only their own); tie `:assignment_id` to `:course_id`.
- [ ] **Gap-1 (LTI AGS)** — give `lti_line_item` repo the `(ctx, id, accountID)` signature; in `GetLineItem`/`UpdateLineItem`/`PostScore`/`GetResults` replicate `DeleteLineItem`'s parent-tie (`item.CourseID != urlCourseID → 404`) and validate `input.UserID` is enrolled in `:course_id`.

**P1 — before public release (hardening + the rest of the High tier):**
- [ ] **Gap-3** — add `RequireInstructor` (or an explicit policy) to `/ai_assist/:action` and a server-side `len(input.Text)` cap before the upstream call.
- [ ] **Gap-2** — thread `accountID` into NRPS and gate roster access to teacher/TA (or LTI-launch context), not any enrolled student.
- [ ] **Gap-4** — round-trip `SharedSecret` (and any LTI 1.3 private keys) through `secretbox.Encrypt`; add the backfill migration.
- [ ] Session cookies: set `Secure` on every federated/MFA/passkey mint path; SAML: enforce `Conditions`/audience/expiry + `InResponseTo`; sanitize the QTI/IMSCC import + portfolio/collaboration URL paths.
- [ ] **CI gate** — `.github/workflows/security.yml` added (govulncheck + gitleaks blocking; gosec + Trivy + npm audit advisory→promote). Harden `Dockerfile.backend` to non-root (see E.4).
- [ ] Add regression tests mirroring `tenant_isolation_test.go` for each fixed handler (cross-student and cross-tenant 404 assertions).

**P2 — follow-up (structure + the medium/low triage list):**
- [ ] Work Appendix B's Medium/Low/Info as a triage backlog (single-validator tier — confirm before fixing).
- [ ] God-file decomposition per §6 / Appendix C (`api.js`, `main.go`, `router.go`, `GradebookPage.jsx`).
- [ ] Audit the remaining critic gaps in Appendix D (masquerade audit actor, webhook signing, mass-assignment sweep, WebAuthn verification, scheduler tenant scope, pairing-code entropy).

### E.4 Docker non-root hardening (`deployments/docker/Dockerfile.backend`)

The runtime stage has no `USER` directive, so the container runs as root. Add a non-root user in the run stage:

```dockerfile
# Run stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /paper-lms .
COPY --from=builder /migrate .
USER app
EXPOSE 3000
```

### E.5 CodeBase scanning note

GitHub **CodeQL** (Go + JavaScript) is best enabled via repository Settings → Code security → "Default setup" rather than a hand-rolled workflow (the default setup avoids conflicting with the SARIF uploads in `security.yml`). Recommended before the repo goes public.
