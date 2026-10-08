package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SubmissionGradedCallback is invoked asynchronously after a submission is
// successfully graded. Callbacks receive a detached context and the graded
// submission's ID. Implementations MUST NOT block grading and MUST NOT panic;
// panics are recovered and logged but never propagated.
type SubmissionGradedCallback func(ctx context.Context, submissionID uint)

// recoverFromPanic recovers from a panic in an OnGraded callback, logging the
// panic value with the originating callback name. Used as `defer
// recoverFromPanic("...")` inside the goroutine that fires each callback so a
// crashing callback never crashes the grading flow.
func recoverFromPanic(label string) {
	if r := recover(); r != nil {
		slog.Error("panic recovered in callback", "label", label, "panic", r)
	}
}

type SubmissionService struct {
	submissionRepo         repository.SubmissionRepository
	assignmentRepo         repository.AssignmentRepository
	enrollmentRepo         repository.EnrollmentRepository
	latePolicyRepo         repository.LatePolicyRepository
	courseRepo             repository.CourseRepository
	gradingPeriodGroupRepo repository.GradingPeriodGroupRepository
	gradingPeriodRepo      repository.GradingPeriodRepository
	groupMembershipRepo    repository.GroupMembershipRepository

	// Optional collaborators wired after construction (see Set* below).
	// All are nil-safe: without them the assignment's own dates apply and
	// grade changes are not audited.
	overrideService      *OverrideService
	accommodationService *AccommodationService
	gradeChangeLogger    GradeChangeLogger
	// Grading-policy collaborators (nil-safe): scheme lookup for letter /
	// GPA grades, attempt history, and the course's grading periods.
	gradingStandardRepo repository.GradingStandardRepository
	policyRepo          repository.GradingPolicyRepository
	periodResolver      CoursePeriodResolver

	// onGradedCallbacks fire (in goroutines) after a successful Grade(...).
	// Registered via OnGraded; never invoked in tests unless explicitly wired.
	onGradedCallbacks []SubmissionGradedCallback
}

// OnGraded registers a callback to fire once a grade is visible to the
// student: after a grade write on a posted submission (automatic post
// policy, or an already-posted one), and when hidden grades are posted
// (FirePosted). A grade still hidden by a manual post policy never fires —
// mastery paths, gamification and outcome results would otherwise let the
// student infer it before release. The callback runs in a fresh goroutine
// with a detached context.Background(), so it MUST be self-contained
// (don't rely on the request's context.Cancel). Multiple registrations
// stack; order is registration order.
func (s *SubmissionService) OnGraded(cb SubmissionGradedCallback) {
	s.onGradedCallbacks = append(s.onGradedCallbacks, cb)
}

// GradeChange describes one grade mutation for the grade-change audit log.
type GradeChange struct {
	CourseID     uint
	AssignmentID uint
	StudentID    uint
	GraderID     uint
	SubmissionID uint
	OldGrade     string
	NewGrade     string
	OldScore     *float64
	NewScore     *float64
	Excused      bool
}

// GradeChangeLogger records a grade change (grade_change_log + audit log).
// Errors are the logger's responsibility; grading never fails on them.
type GradeChangeLogger func(ctx context.Context, change GradeChange)

// SetOverrideService wires assignment overrides into lock/unlock/late
// resolution so per-student and per-section dates are honored.
func (s *SubmissionService) SetOverrideService(o *OverrideService) { s.overrideService = o }

// SetAccommodationService wires per-student extra-day accommodations
// into the effective due/lock dates.
func (s *SubmissionService) SetAccommodationService(a *AccommodationService) {
	s.accommodationService = a
}

// SetGradeChangeLogger registers the grade-change audit sink.
func (s *SubmissionService) SetGradeChangeLogger(l GradeChangeLogger) { s.gradeChangeLogger = l }

// SetGradingStandardRepository enables grading-scheme conversions
// (letter / GPA grades) when grading.
func (s *SubmissionService) SetGradingStandardRepository(r repository.GradingStandardRepository) {
	s.gradingStandardRepo = r
}

// SetGradingPolicyRepository enables submission attempt history.
func (s *SubmissionService) SetGradingPolicyRepository(r repository.GradingPolicyRepository) {
	s.policyRepo = r
}

// SetPeriodResolver scopes grading-period locks to the course's own
// grading period set.
func (s *SubmissionService) SetPeriodResolver(r CoursePeriodResolver) { s.periodResolver = r }

func (s *SubmissionService) logGradeChange(ctx context.Context, assignment *models.Assignment, sub *models.Submission, graderID uint, oldGrade *string, oldScore *float64) {
	if s.gradeChangeLogger == nil || assignment == nil || sub == nil {
		return
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	s.gradeChangeLogger(ctx, GradeChange{
		CourseID:     assignment.CourseID,
		AssignmentID: assignment.ID,
		StudentID:    sub.UserID,
		GraderID:     graderID,
		SubmissionID: sub.ID,
		OldGrade:     deref(oldGrade),
		NewGrade:     deref(sub.Grade),
		OldScore:     oldScore,
		NewScore:     sub.Score,
		Excused:      sub.Excused,
	})
}

// EffectiveDates returns the due/unlock/lock dates that apply to userID
// on assignment: the assignment's dates, replaced by the most lenient
// applicable override, then extended by the student's extra-days
// accommodation (if any).
func (s *SubmissionService) EffectiveDates(ctx context.Context, assignment *models.Assignment, userID uint) (dueAt, unlockAt, lockAt *time.Time) {
	dueAt, unlockAt, lockAt, _ = s.EffectiveDatesAssigned(ctx, assignment, userID)
	return
}

// EffectiveDatesAssigned is EffectiveDates plus whether the assignment is
// assigned to the student at all (an only_visible_to_overrides
// assignment is assigned only through an override).
func (s *SubmissionService) EffectiveDatesAssigned(ctx context.Context, assignment *models.Assignment, userID uint) (dueAt, unlockAt, lockAt *time.Time, assigned bool) {
	if assignment == nil {
		return nil, nil, nil, false
	}
	hasOverride := false
	if s.overrideService != nil {
		dueAt, unlockAt, lockAt, hasOverride = s.overrideService.EffectiveDatesWithOverride(ctx, assignment, userID)
	} else {
		dueAt, unlockAt, lockAt = assignment.DueAt, assignment.UnlockAt, assignment.LockAt
	}
	assigned = !assignment.OnlyVisibleToOverrides || hasOverride
	if s.accommodationService != nil && dueAt != nil {
		courseID := assignment.CourseID
		if adj, err := s.accommodationService.ApplyAccommodationsToAssignment(ctx, userID, &courseID, dueAt); err == nil && adj != nil && adj.ExtraDays > 0 {
			dueAt = adj.AdjustedDueAt
			if lockAt != nil {
				extended := lockAt.AddDate(0, 0, adj.ExtraDays)
				lockAt = &extended
			}
		}
	}
	return dueAt, unlockAt, lockAt, assigned
}

// fireOnGraded runs all registered callbacks in goroutines with a detached
// context. Panics are recovered. Errors from callbacks (if any) are the
// callback's responsibility to log — the signature returns no error.
func (s *SubmissionService) fireOnGraded(submissionID uint) {
	if submissionID == 0 {
		return
	}
	for _, cb := range s.onGradedCallbacks {
		go func(cb SubmissionGradedCallback) {
			defer recoverFromPanic("submission OnGraded callback")
			cb(context.Background(), submissionID)
		}(cb)
	}
}

func NewSubmissionService(
	submissionRepo repository.SubmissionRepository,
	assignmentRepo repository.AssignmentRepository,
	enrollmentRepo repository.EnrollmentRepository,
	latePolicyRepo repository.LatePolicyRepository,
	courseRepo repository.CourseRepository,
	gradingPeriodGroupRepo repository.GradingPeriodGroupRepository,
	gradingPeriodRepo repository.GradingPeriodRepository,
	groupMembershipRepo repository.GroupMembershipRepository,
) *SubmissionService {
	return &SubmissionService{
		submissionRepo:         submissionRepo,
		assignmentRepo:         assignmentRepo,
		enrollmentRepo:         enrollmentRepo,
		latePolicyRepo:         latePolicyRepo,
		courseRepo:             courseRepo,
		gradingPeriodGroupRepo: gradingPeriodGroupRepo,
		gradingPeriodRepo:      gradingPeriodRepo,
		groupMembershipRepo:    groupMembershipRepo,
	}
}

// ErrSubmissionLocked is returned when a student attempts to submit
// (or resubmit) after the assignment's lock_at window has closed.
// Handlers map this to 409 Conflict so the UI can show "this assignment
// is locked" instead of a generic 400.
var ErrSubmissionLocked = errors.New("assignment is locked")

// ErrSubmissionNotYetUnlocked is returned when a student attempts to
// submit before the assignment's unlock_at window has opened. Handlers
// map this to 409 Conflict — same shape as ErrSubmissionLocked, distinct
// failure mode so the UI can distinguish "not yet" from "too late".
var ErrSubmissionNotYetUnlocked = errors.New("assignment is not yet available")

// ErrAssignmentUnavailable is returned by Submit when the assignment does
// not exist in the caller's tenant, belongs to a different course than
// the URL, or is not published. Handlers map it to 404 (existence-leak
// contract: an unpublished draft is indistinguishable from a missing one).
var ErrAssignmentUnavailable = errors.New("assignment not found")

// ErrSubmissionTypeNotAllowed is returned when the requested
// submission_type is not one the assignment accepts online.
var ErrSubmissionTypeNotAllowed = errors.New("submission type not allowed for this assignment")

// onlineSubmissionTypes are the submission types a student can create
// through the submissions API. Everything else (on_paper, none,
// external_tool, online_quiz, discussion_topic, not_graded, ...) is
// either not submittable online or produced by its own subsystem.
var onlineSubmissionTypes = map[string]bool{
	"online_text_entry":  true,
	"online_url":         true,
	"online_upload":      true,
	"media_recording":    true,
	"student_annotation": true,
}

// SubmissionTypeAllowed reports whether subType may be submitted online
// to assignment.
func SubmissionTypeAllowed(assignment *models.Assignment, subType string) bool {
	if assignment == nil || !onlineSubmissionTypes[subType] {
		return false
	}
	for _, t := range strings.Split(assignment.SubmissionTypes, ",") {
		if strings.TrimSpace(t) == subType {
			return true
		}
	}
	return false
}

// Submit is the student-facing create path. It loads the assignment
// under the caller's tenant, ties it to the URL's course, requires it to
// be published and to accept the requested submission type, and then
// applies the per-student lock/unlock/late rules.
func (s *SubmissionService) Submit(ctx context.Context, submission *models.Submission, courseID, accountID uint) error {
	assignment, err := s.assignmentRepo.FindByID(ctx, submission.AssignmentID, accountID)
	if err != nil || assignment.CourseID != courseID || !assignment.IsPublished() {
		return ErrAssignmentUnavailable
	}
	if submission.SubmissionType == nil || *submission.SubmissionType == "" {
		return errors.New("submission_type is required")
	}
	if !SubmissionTypeAllowed(assignment, *submission.SubmissionType) {
		return ErrSubmissionTypeNotAllowed
	}
	switch *submission.SubmissionType {
	case "online_text_entry":
		if submission.Body == nil || strings.TrimSpace(*submission.Body) == "" {
			return errors.New("body is required for a text entry submission")
		}
	case "online_url":
		if submission.URL == nil || strings.TrimSpace(*submission.URL) == "" {
			return errors.New("url is required for a website URL submission")
		}
		// The URL is rendered as a link in SpeedGrader, the student's
		// page and the peer-review view; a javascript:/data: URL would run
		// in the grader's session when clicked.
		u, err := url.Parse(strings.TrimSpace(*submission.URL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("url must be an http or https link")
		}
		clean := u.String()
		submission.URL = &clean
	case "online_upload":
		if submission.Attachments == nil || *submission.Attachments == "" {
			return errors.New("at least one file is required for a file upload submission")
		}
	}
	return s.createForAssignment(ctx, assignment, submission)
}

// Create is the internal (non-HTTP) create path; the assignment is
// loaded without tenant scope because internal callers have already
// verified it. HTTP handlers MUST use Submit.
func (s *SubmissionService) Create(ctx context.Context, submission *models.Submission) error {
	assignment, err := s.assignmentRepo.FindByID(ctx, submission.AssignmentID, 0)
	if err != nil {
		return errors.New("assignment not found")
	}
	if submission.SubmissionType == nil || *submission.SubmissionType == "" {
		return errors.New("submission_type is required")
	}
	return s.createForAssignment(ctx, assignment, submission)
}

func (s *SubmissionService) createForAssignment(ctx context.Context, assignment *models.Assignment, submission *models.Submission) error {
	// B002: online_text_entry bodies are HTML rendered in SpeedGrader and on
	// the assignment page, so sanitize at the single submit chokepoint (also
	// covers the copies made for group members below).
	if submission.Body != nil && *submission.Body != "" {
		clean := SanitizeHTML(*submission.Body)
		submission.Body = &clean
	}

	now := time.Now()

	// F-047 / B022: unlock_at / lock_at windows, resolved per student so
	// assignment overrides and extra-day accommodations are honored.
	dueAt, unlockAt, lockAt, assigned := s.EffectiveDatesAssigned(ctx, assignment, submission.UserID)
	if !assigned {
		// G019: assigned to specific students/sections only.
		return ErrAssignmentUnavailable
	}
	if unlockAt != nil && now.Before(*unlockAt) {
		return ErrSubmissionNotYetUnlocked
	}
	if lockAt != nil && now.After(*lockAt) {
		return ErrSubmissionLocked
	}

	submission.SubmittedAt = &now
	submission.Attempt = 1
	submission.WorkflowState = models.SubmissionSubmitted
	submission.CachedDueDate = dueAt

	// Check for existing submission and increment attempt.
	// accountID=0: the assignment was already loaded (tenant-scoped on the
	// HTTP path) and (assignment_id, user_id) identifies the row.
	existing, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, submission.AssignmentID, submission.UserID, 0)
	if existing != nil {
		existing.SubmissionType = submission.SubmissionType
		existing.Body = submission.Body
		existing.URL = submission.URL
		existing.Attachments = submission.Attachments
		existing.SubmittedAt = &now
		existing.Attempt = existing.Attempt + 1
		existing.CachedDueDate = dueAt
		existing.WorkflowState = "submitted"
		*submission = *existing
	}
	markSubmitted(assignment, submission, dueAt, now)
	if existing != nil {
		if err := s.submissionRepo.Update(ctx, submission); err != nil {
			return err
		}
	} else {
		if err := s.submissionRepo.Create(ctx, submission); err != nil {
			return err
		}
	}
	s.recordAttempt(ctx, submission)

	// For group assignments, create/update submissions for all other group members
	if assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 {
		s.createGroupSubmissions(ctx, assignment, submission)
	}

	return nil
}

// GetByAssignmentAndUser is the cross-tenant-sensitive read path.
// callers MUST pass `callerAccountID(c)` from a handler; 0 from internal
// callers (LTI callbacks, peer-review wiring) where the (assignment,
// user) pair was already tenant-verified upstream.
func (s *SubmissionService) GetByAssignmentAndUser(ctx context.Context, assignmentID, userID, accountID uint) (*models.Submission, error) {
	return s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID, accountID)
}

// ErrSubmissionNotFound is returned when an operation needs an existing
// submission row and there is none.
var ErrSubmissionNotFound = errors.New("submission not found")

// IsCourseStudent reports whether userID holds an active StudentEnrollment
// in courseID (tenant-scoped by accountID). Grading endpoints use it so a
// grade can't be minted for a user who isn't in the course.
func (s *SubmissionService) IsCourseStudent(ctx context.Context, courseID, userID, accountID uint) bool {
	if s.enrollmentRepo == nil {
		return false
	}
	e, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	return err == nil && e != nil && e.Type == "StudentEnrollment"
}

// SetExcused flips the excused flag on a submission and persists the
// change, creating the submission row when an instructor excuses a
// student who never submitted (Canvas allows this). The assignment is
// loaded under accountID so a cross-tenant id returns ErrGradeCrossTenant.
func (s *SubmissionService) SetExcused(ctx context.Context, assignmentID, userID, graderID, accountID uint, excused bool) (*models.Submission, error) {
	assignment, err := s.assignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil {
		return nil, ErrGradeCrossTenant
	}
	if closed, periodTitle := s.isGradingPeriodClosedFor(ctx, assignment, userID); closed {
		return nil, fmt.Errorf("grading period %q is closed — grades cannot be modified", periodTitle)
	}
	now := time.Now()

	submission, findErr := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID, accountID)
	isNew := findErr != nil || submission == nil
	if isNew {
		if !excused {
			// Nothing to un-excuse.
			return nil, ErrSubmissionNotFound
		}
		submission = &models.Submission{
			AssignmentID:  assignmentID,
			UserID:        userID,
			WorkflowState: models.SubmissionUnsubmitted,
		}
	}
	oldGrade, oldScore := submission.Grade, submission.Score

	submission.Excused = excused
	submission.GraderID = &graderID
	submission.GradedAt = &now
	if excused {
		// G021 / Canvas: excusing removes the grade and any manual
		// status; an excused submission counts as graded.
		submission.Score = nil
		submission.Grade = nil
		submission.EnteredScore = nil
		submission.EnteredGrade = nil
		submission.PointsDeducted = nil
		submission.LatePolicyStatus = nil
		submission.SecondsLateOverride = nil
		submission.Late = false
		submission.Missing = false
		submission.WorkflowState = models.SubmissionGraded
		if !assignment.UsesManualPostPolicy() && submission.PostedAt == nil {
			submission.PostedAt = &now
		}
	} else if submission.Score == nil {
		if submission.SubmittedAt != nil {
			submission.WorkflowState = models.SubmissionSubmitted
		} else {
			submission.WorkflowState = models.SubmissionUnsubmitted
		}
		due, _, _ := s.EffectiveDates(ctx, assignment, userID)
		submission.CachedDueDate = due
		t := ComputeTardiness(assignment, submission, due, now)
		submission.Late, submission.Missing = t.Late, t.Missing
	}

	if isNew {
		if err := s.submissionRepo.Create(ctx, submission); err != nil {
			return nil, err
		}
	} else if err := s.submissionRepo.Update(ctx, submission); err != nil {
		return nil, err
	}
	s.logGradeChange(ctx, assignment, submission, graderID, oldGrade, oldScore)
	return submission, nil
}

func (s *SubmissionService) ListByAssignment(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	return s.submissionRepo.ListByAssignmentID(ctx, assignmentID, params)
}

func (s *SubmissionService) BulkListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	return s.submissionRepo.BulkListByCourse(ctx, courseID, params)
}

// ListByUserAndCourse returns one student's submissions across the course
// (bounded by the number of assignments). Used by the course submissions
// endpoint when the caller asks for a single student, so the filter runs in
// SQL rather than after paginating the whole class.
func (s *SubmissionService) ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.Submission, error) {
	return s.submissionRepo.ListByUserAndCourse(ctx, userID, courseID)
}

func (s *SubmissionService) PostGradesByAssignment(ctx context.Context, assignmentID uint, postedAt *time.Time) error {
	return s.submissionRepo.PostGradesByAssignment(ctx, assignmentID, postedAt)
}

// fireIfPosted fires the OnGraded callbacks for a graded submission the
// student can see. Hidden (unposted) grades fire later, from FirePosted.
func (s *SubmissionService) fireIfPosted(sub *models.Submission) {
	if sub != nil && sub.PostedAt != nil {
		s.fireOnGraded(sub.ID)
	}
}

// FirePosted fires the OnGraded callbacks for submissions whose grades were
// just released by posting (PostGrades), so everything that reacts to a
// visible grade runs at release time.
func (s *SubmissionService) FirePosted(submissionIDs ...uint) {
	for _, id := range submissionIDs {
		s.fireOnGraded(id)
	}
}

// MarkPosted posts a single submission (sets posted_at) if it is not
// already posted. Used when an instructor comments on a submission of an
// automatically-posted assignment: Canvas posts the submission then.
func (s *SubmissionService) MarkPosted(ctx context.Context, submission *models.Submission) error {
	if submission == nil || submission.PostedAt != nil {
		return nil
	}
	now := time.Now()
	submission.PostedAt = &now
	return s.submissionRepo.Update(ctx, submission)
}

// ErrGradeCrossTenant is returned when a Grade call attempts to score
// a submission whose assignment lives outside the grader's tenant.
// Handlers map this to 404 (existence-leak contract).
var ErrGradeCrossTenant = errors.New("assignment not in grader's tenant")

// Grade scores an existing submission (or creates a graded submission
// for a student who hadn't submitted yet, the "teacher grades a missing
// student" path). An empty postedGrade clears the grade.
//
// postedGrade is the ENTERED score. The course late policy is applied to
// it to produce the final Score; the entered value is kept in
// EnteredScore so re-saving, curving or re-importing a grade never
// compounds the late penalty (Canvas entered_score semantics).
//
// Post policy: on an automatically-posted assignment the graded
// submission is posted (posted_at set) immediately; on a manual one it
// stays hidden from the student until an instructor posts grades.
//
// SECURITY (F-016): callerAccountID gates both the assignment and the
// existing-submission lookups. The pre-fix path passed accountID=0
// through every repo call, which let a teacher in course A submit:
//
//	PUT /courses/A/assignments/<id from tenant B>/submissions/<user>
//
// and rewrite (or CREATE) a graded submission for any student in any
// tenant. The fix loads the assignment under the caller's tenant; an
// assignment in a different tenant returns ErrGradeCrossTenant. The
// course parent-tie and student-enrollment checks live in the handler.
func (s *SubmissionService) Grade(ctx context.Context, assignmentID, userID, graderID, callerAccountID uint, postedGrade string) (*models.Submission, error) {
	postedGrade = strings.TrimSpace(postedGrade)
	clearing := postedGrade == ""

	// G021: "EX" in a grade box excuses the student (Canvas gradebook).
	if strings.EqualFold(postedGrade, "EX") || strings.EqualFold(postedGrade, "excused") {
		return s.SetExcused(ctx, assignmentID, userID, graderID, callerAccountID, true)
	}

	// Tenant gate — assignment lookup MUST scope to the caller's
	// tenant. A miss = cross-tenant grading attempt = refuse.
	assignment, aErr := s.assignmentRepo.FindByID(ctx, assignmentID, callerAccountID)
	if aErr != nil {
		return nil, ErrGradeCrossTenant
	}

	// G023: the typed grade is read per the assignment's grading type
	// (points, "85%", a letter or GPA grade, complete/incomplete).
	var entered float64
	if !clearing {
		v, err := InterpretGrade(assignment, s.resolveScheme(ctx, assignment), postedGrade)
		if err != nil {
			return nil, err
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("invalid grade value: %s", postedGrade)
		}
		entered = v
	}

	// Check if grading period is closed for this assignment
	if closed, periodTitle := s.isGradingPeriodClosedFor(ctx, assignment, userID); closed {
		return nil, fmt.Errorf("grading period %q is closed — grades cannot be modified", periodTitle)
	}

	now := time.Now()

	submission, findErr := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, userID, callerAccountID)
	isNew := findErr != nil || submission == nil

	if clearing {
		if isNew {
			// Nothing graded yet — clearing is a no-op.
			return &models.Submission{AssignmentID: assignmentID, UserID: userID, WorkflowState: models.SubmissionUnsubmitted}, nil
		}
		oldGrade, oldScore := submission.Grade, submission.Score
		clearGrade(submission, graderID)
		if err := s.submissionRepo.Update(ctx, submission); err != nil {
			return nil, err
		}
		s.logGradeChange(ctx, assignment, submission, graderID, oldGrade, oldScore)
		// A group grade was applied to every member, so clearing it
		// clears it for every member too.
		if assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 {
			for _, memberID := range s.getGroupMemberIDs(ctx, userID, *assignment.GroupCategoryID) {
				member, err := s.submissionRepo.FindByAssignmentAndUser(ctx, assignmentID, memberID, callerAccountID)
				if err != nil || member == nil || (member.Score == nil && member.Grade == nil) {
					continue
				}
				mOldGrade, mOldScore := member.Grade, member.Score
				clearGrade(member, graderID)
				if err := s.submissionRepo.Update(ctx, member); err == nil {
					s.logGradeChange(ctx, assignment, member, graderID, mOldGrade, mOldScore)
				}
			}
		}
		return submission, nil
	}

	var oldGrade *string
	var oldScore *float64
	if isNew {
		// No existing submission — create one (teacher grading without student submission)
		submission = &models.Submission{
			AssignmentID: assignmentID,
			UserID:       userID,
		}
	} else {
		oldGrade, oldScore = submission.Grade, submission.Score
	}

	s.applyGradeGiven(ctx, assignment, submission, entered, postedGrade, graderID, now)

	if isNew {
		if createErr := s.submissionRepo.Create(ctx, submission); createErr != nil {
			return nil, createErr
		}
	} else if err := s.submissionRepo.Update(ctx, submission); err != nil {
		return nil, err
	}
	s.recordAttemptGrade(ctx, submission)
	s.logGradeChange(ctx, assignment, submission, graderID, oldGrade, oldScore)

	// For group assignments, apply the same grade to all other group members.
	// Tenant is already enforced via the assignment load above.
	if assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 {
		s.gradeGroupMembers(ctx, assignment, userID, graderID, entered, now)
	}

	// Fire post-grade callbacks (mastery paths, etc.) asynchronously, but
	// only once the grade is visible to the student (see OnGraded).
	// Failures in callbacks must never block grading or surface here.
	s.fireIfPosted(submission)

	return submission, nil
}

// clearGrade removes a submission's grade (B113): score, grade, entered
// score and deduction are nulled and the state falls back to submitted
// or unsubmitted.
func clearGrade(sub *models.Submission, graderID uint) {
	grader := graderID
	sub.Score = nil
	sub.Grade = nil
	sub.EnteredScore = nil
	sub.EnteredGrade = nil
	sub.PointsDeducted = nil
	sub.GradedAt = nil
	sub.GraderID = &grader
	if sub.SubmittedAt != nil {
		sub.WorkflowState = models.SubmissionSubmitted
	} else {
		sub.WorkflowState = models.SubmissionUnsubmitted
	}
}

// applyGrade writes an entered score onto sub: the late-policy deduction
// is computed from the entered value (never from a previously deducted
// score), and the submission is posted when the assignment posts
// automatically.
func (s *SubmissionService) applyGrade(ctx context.Context, assignment *models.Assignment, sub *models.Submission, entered float64, graderID uint, now time.Time) {
	s.applyGradeGiven(ctx, assignment, sub, entered, "", graderID, now)
}

// applyGradeGiven is applyGrade with the grade the grader typed, which
// labels point-less letter / pass-fail grades.
func (s *SubmissionService) applyGradeGiven(ctx context.Context, assignment *models.Assignment, sub *models.Submission, entered float64, given string, graderID uint, now time.Time) {
	deducted := s.latePointsDeducted(ctx, assignment, sub, entered)
	final := entered - deducted
	enteredScore := entered
	// G023: grades are stored in the assignment's grading type ("B+",
	// "85%", "complete", or points).
	scheme := s.resolveScheme(ctx, assignment)
	enteredGrade := FormatGrade(assignment, scheme, entered, given)
	finalGrade := FormatGrade(assignment, scheme, final, given)
	gradedAt := now
	grader := graderID

	sub.EnteredScore = &enteredScore
	sub.EnteredGrade = &enteredGrade
	if deducted > 0 {
		sub.PointsDeducted = &deducted
	} else {
		sub.PointsDeducted = nil
	}
	sub.Score = &final
	sub.Grade = &finalGrade
	sub.GradedAt = &gradedAt
	sub.GraderID = &grader
	sub.WorkflowState = models.SubmissionGraded
	// Canvas: entering a grade on an excused submission un-excuses it.
	sub.Excused = false
	// A graded submission is no longer missing unless an instructor set
	// the missing status explicitly.
	if sub.LatePolicyStatus == nil || *sub.LatePolicyStatus != models.LatePolicyStatusMissing {
		sub.Missing = false
	}
	if !assignment.UsesManualPostPolicy() && sub.PostedAt == nil {
		postedAt := now
		sub.PostedAt = &postedAt
	}
}

// getGroupMemberIDs returns the user IDs of all members in the same group as the given user
// within the specified group category, excluding the given user.
func (s *SubmissionService) getGroupMemberIDs(ctx context.Context, userID, groupCategoryID uint) []uint {
	if s.groupMembershipRepo == nil {
		return nil
	}

	// Find the user's group in this category
	group, err := s.groupMembershipRepo.FindUserGroupInCategory(ctx, userID, groupCategoryID)
	if err != nil {
		return nil
	}

	// List all members of that group
	memberships, err := s.groupMembershipRepo.ListByGroupID(ctx, group.ID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil
	}

	var memberIDs []uint
	for _, m := range memberships.Items {
		if m.UserID != userID && m.WorkflowState == "accepted" {
			memberIDs = append(memberIDs, m.UserID)
		}
	}
	return memberIDs
}

// createGroupSubmissions creates or updates submissions for all other group members
// when a student submits a group assignment.
func (s *SubmissionService) createGroupSubmissions(ctx context.Context, assignment *models.Assignment, submission *models.Submission) {
	memberIDs := s.getGroupMemberIDs(ctx, submission.UserID, *assignment.GroupCategoryID)
	for _, memberID := range memberIDs {
		memberDue, _, _ := s.EffectiveDates(ctx, assignment, memberID)
		submittedAt := time.Now()
		if submission.SubmittedAt != nil {
			submittedAt = *submission.SubmittedAt
		}
		existing, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, submission.AssignmentID, memberID, 0)
		if existing != nil {
			existing.SubmissionType = submission.SubmissionType
			existing.Body = submission.Body
			existing.URL = submission.URL
			existing.Attachments = submission.Attachments
			existing.SubmittedAt = submission.SubmittedAt
			existing.Attempt = existing.Attempt + 1
			existing.CachedDueDate = memberDue
			existing.WorkflowState = "submitted"
			markSubmitted(assignment, existing, memberDue, submittedAt)
			if err := s.submissionRepo.Update(ctx, existing); err == nil {
				s.recordAttempt(ctx, existing)
			}
		} else {
			memberSubmission := &models.Submission{
				AssignmentID:   submission.AssignmentID,
				UserID:         memberID,
				SubmissionType: submission.SubmissionType,
				Body:           submission.Body,
				URL:            submission.URL,
				Attachments:    submission.Attachments,
				SubmittedAt:    submission.SubmittedAt,
				Attempt:        1,
				CachedDueDate:  memberDue,
				WorkflowState:  models.SubmissionSubmitted,
			}
			markSubmitted(assignment, memberSubmission, memberDue, submittedAt)
			if err := s.submissionRepo.Create(ctx, memberSubmission); err == nil {
				s.recordAttempt(ctx, memberSubmission)
			}
		}
	}
}

// gradeGroupMembers applies the same entered grade to all other group
// members when grading a group assignment submission. The caller has
// already loaded the assignment under its tenant scope; the per-member
// late deduction is computed from each member's own submission (their
// lateness can differ when overrides or accommodations apply).
func (s *SubmissionService) gradeGroupMembers(ctx context.Context, assignment *models.Assignment, gradedUserID, graderID uint, entered float64, gradedAt time.Time) {
	memberIDs := s.getGroupMemberIDs(ctx, gradedUserID, *assignment.GroupCategoryID)
	// Resolve the assignment's tenant scope from its course (assignment
	// rows don't carry account_id directly).
	tenantScope := uint(0)
	if course, cErr := s.courseRepo.FindByID(ctx, assignment.CourseID, 0); cErr == nil {
		tenantScope = course.AccountID
	}
	for _, memberID := range memberIDs {
		existing, _ := s.submissionRepo.FindByAssignmentAndUser(ctx, assignment.ID, memberID, tenantScope)
		if existing != nil {
			oldGrade, oldScore := existing.Grade, existing.Score
			s.applyGrade(ctx, assignment, existing, entered, graderID, gradedAt)
			if err := s.submissionRepo.Update(ctx, existing); err == nil {
				s.recordAttemptGrade(ctx, existing)
				s.logGradeChange(ctx, assignment, existing, graderID, oldGrade, oldScore)
				s.fireIfPosted(existing)
			}
		} else {
			memberSubmission := &models.Submission{
				AssignmentID: assignment.ID,
				UserID:       memberID,
			}
			s.applyGrade(ctx, assignment, memberSubmission, entered, graderID, gradedAt)
			if err := s.submissionRepo.Create(ctx, memberSubmission); err == nil {
				s.logGradeChange(ctx, assignment, memberSubmission, graderID, nil, nil)
				s.fireIfPosted(memberSubmission)
			}
		}
	}
}

// pointDeductibleGradingTypes mirrors Canvas LatePolicy::POINT_DEDUCTIBLE_GRADING_TYPES:
// pass/fail and not-graded assignments never take a late deduction.
var pointDeductibleGradingTypes = map[string]bool{
	"points":       true,
	"percent":      true,
	"letter_grade": true,
	"gpa_scale":    true,
}

// LatePointsDeducted is Canvas's LatePolicy#points_deducted:
//
//	intervals      = ceil(late_for / interval)
//	raw_percent    = score / possible * 100
//	maximum_deduct = max(raw_percent - minimum_percent, 0)
//	deduction      = possible * min(deduction_per_interval * intervals, maximum_deduct) / 100
//
// The deduction is a percentage of points POSSIBLE (not of the earned
// score), and the minimum-percent floor only caps the deduction; it never
// raises a low raw score.
func LatePointsDeducted(policy *models.LatePolicy, score, possible float64, lateFor time.Duration) float64 {
	if policy == nil || !policy.LateSubmissionDeductionEnabled || possible <= 0 || lateFor <= 0 {
		return 0
	}
	interval := 24 * time.Hour
	if policy.LateSubmissionInterval == "hour" {
		interval = time.Hour
	}
	intervals := math.Ceil(float64(lateFor) / float64(interval))
	minimumPercent := 0.0
	if policy.LateSubmissionMinimumPercentEnabled {
		minimumPercent = policy.LateSubmissionMinimumPercent
	}
	rawScorePercent := score * 100.0 / possible
	maximumDeduct := math.Max(rawScorePercent-minimumPercent, 0)
	latePercentDeduct := policy.LateSubmissionDeduction * intervals
	deducted := possible * math.Min(latePercentDeduct, maximumDeduct) / 100
	// Strip float noise (e.g. 1.0000000000000002) without losing real precision.
	return math.Round(deducted*1e6) / 1e6
}

// latePointsDeducted returns how many points the course late policy takes
// off an entered score for this submission. Lateness is measured against
// the student's effective due date (overrides + accommodations).
func (s *SubmissionService) latePointsDeducted(ctx context.Context, assignment *models.Assignment, sub *models.Submission, entered float64) float64 {
	if assignment == nil || sub == nil || s.latePolicyRepo == nil {
		return 0
	}
	// G022: a manual status decides lateness. "late" uses the
	// instructor's seconds-late override; none / extended / missing
	// take no late deduction.
	if sub.LatePolicyStatus != nil {
		if *sub.LatePolicyStatus != models.LatePolicyStatusLate {
			return 0
		}
		gradingType := assignment.GradingType
		if gradingType == "" {
			gradingType = "points"
		}
		if !pointDeductibleGradingTypes[gradingType] || assignment.PointsPossible == nil || *assignment.PointsPossible <= 0 ||
			sub.SecondsLateOverride == nil || *sub.SecondsLateOverride <= 0 {
			return 0
		}
		policy, err := s.latePolicyRepo.FindByCourseID(ctx, assignment.CourseID)
		if err != nil || policy == nil {
			return 0
		}
		return LatePointsDeducted(policy, entered, *assignment.PointsPossible, time.Duration(*sub.SecondsLateOverride)*time.Second)
	}
	if sub.SubmittedAt == nil {
		return 0
	}
	gradingType := assignment.GradingType
	if gradingType == "" {
		gradingType = "points"
	}
	if !pointDeductibleGradingTypes[gradingType] {
		return 0
	}
	if assignment.PointsPossible == nil || *assignment.PointsPossible <= 0 {
		return 0
	}
	dueAt, _, _ := s.EffectiveDates(ctx, assignment, sub.UserID)
	if dueAt == nil {
		return 0
	}
	lateFor := sub.SubmittedAt.Sub(*dueAt)
	if lateFor <= 0 {
		return 0
	}
	policy, err := s.latePolicyRepo.FindByCourseID(ctx, assignment.CourseID)
	if err != nil || policy == nil {
		return 0
	}
	return LatePointsDeducted(policy, entered, *assignment.PointsPossible, lateFor)
}

// isGradingPeriodClosed checks whether the given assignment falls
// within a closed grading period. Takes the already-loaded assignment
// so the caller's tenant scope (verified at the original FindByID
// site) is preserved without an extra unscoped lookup.
func (s *SubmissionService) isGradingPeriodClosed(ctx context.Context, assignment *models.Assignment) (bool, string) {
	if assignment == nil {
		return false, ""
	}

	// Use assignment due date to determine which grading period applies
	dueAt := assignment.DueAt
	if dueAt == nil {
		// No due date means no grading period applies
		return false, ""
	}

	course, err := s.courseRepo.FindByID(ctx, assignment.CourseID, 0)
	if err != nil {
		return false, ""
	}

	// Find grading period groups for the course's account
	groups, err := s.gradingPeriodGroupRepo.ListByAccountID(ctx, course.AccountID, repository.PaginationParams{Page: 1, PerPage: 100})
	if err != nil || len(groups.Items) == 0 {
		return false, ""
	}

	now := time.Now()

	for _, group := range groups.Items {
		if group.WorkflowState != "active" {
			continue
		}
		periods, err := s.gradingPeriodRepo.ListByGroupID(ctx, group.ID, 0)
		if err != nil {
			continue
		}
		for _, period := range periods {
			if period.WorkflowState != "active" {
				continue
			}
			// Check if assignment due date falls within this period
			if dueAt.After(period.StartDate) && (dueAt.Before(period.EndDate) || dueAt.Equal(period.EndDate)) {
				// Period contains this assignment — check if it's closed
				if period.IsClosed {
					return true, period.Title
				}
				// Check CloseDate if set
				if period.CloseDate != nil && now.After(*period.CloseDate) {
					return true, period.Title
				}
			}
		}
	}

	return false, ""
}
