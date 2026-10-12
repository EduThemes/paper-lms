package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type SubmissionHandler struct {
	submissionService           *service.SubmissionService
	commentRepo                 repository.SubmissionCommentRepository
	attachmentRepo              repository.AttachmentRepository
	userRepo                    repository.UserRepository
	assignmentRepo              repository.AssignmentRepository
	notificationDeliveryService *service.NotificationDeliveryService
	observerService             *service.ObserverService
	outcomeAlignmentRepo        repository.OutcomeAlignmentRepository
	outcomeService              *service.LearningOutcomeService
	auditService                *service.AuditService

	// Optional, wired with setters (nil-safe).
	fileService       *service.FileService
	peerReviewService *service.PeerReviewService
	gradingPolicy     *service.GradingPolicyService
	gradingUI         *service.GradingUIService
}

// SetGradingPolicyService enables post / hide by section and graded-only
// posting (G018).
func (h *SubmissionHandler) SetGradingPolicyService(p *service.GradingPolicyService) {
	h.gradingPolicy = p
}

func NewSubmissionHandler(submissionService *service.SubmissionService, commentRepo repository.SubmissionCommentRepository, attachmentRepo repository.AttachmentRepository, userRepo repository.UserRepository, assignmentRepo repository.AssignmentRepository, notificationDeliveryService *service.NotificationDeliveryService, observerService *service.ObserverService, outcomeAlignmentRepo repository.OutcomeAlignmentRepository, outcomeService *service.LearningOutcomeService, auditService *service.AuditService) *SubmissionHandler {
	return &SubmissionHandler{
		submissionService:           submissionService,
		commentRepo:                 commentRepo,
		attachmentRepo:              attachmentRepo,
		userRepo:                    userRepo,
		assignmentRepo:              assignmentRepo,
		notificationDeliveryService: notificationDeliveryService,
		observerService:             observerService,
		outcomeAlignmentRepo:        outcomeAlignmentRepo,
		outcomeService:              outcomeService,
		auditService:                auditService,
	}
}

// SetFileService enables the student submission-file upload/download
// endpoints.
func (h *SubmissionHandler) SetFileService(fs *service.FileService) { h.fileService = fs }

// SetGradingUIService enables submission statuses, group comments and
// the other grading-UI features (nil-safe).
func (h *SubmissionHandler) SetGradingUIService(g *service.GradingUIService) { h.gradingUI = g }

// SetPeerReviewService lets an assigned peer reviewer read the work they
// were asked to review.
func (h *SubmissionHandler) SetPeerReviewService(prs *service.PeerReviewService) {
	h.peerReviewService = prs
}

// submissionToJSON is the full (staff) representation.
func submissionToJSON(s *models.Submission) fiber.Map {
	result := fiber.Map{
		"id":              s.ID,
		"assignment_id":   s.AssignmentID,
		"user_id":         s.UserID,
		"submission_type": s.SubmissionType,
		"body":            s.Body,
		"url":             s.URL,
		"score":           s.Score,
		"grade":           s.Grade,
		"entered_score":   s.EnteredScore,
		"entered_grade":   s.EnteredGrade,
		"points_deducted": s.PointsDeducted,
		"graded_at":       s.GradedAt,
		"grader_id":       s.GraderID,
		"submitted_at":    s.SubmittedAt,
		"posted_at":       s.PostedAt,
		"attempt":         s.Attempt,
		"late":            s.Late,
		"missing":         s.Missing,
		"excused":         s.Excused,
		"workflow_state":  s.WorkflowState,
		"preview_url":     nil,
		// Grading policy (G022): manual status and the student's
		// effective due date.
		"late_policy_status":     s.LatePolicyStatus,
		"seconds_late_override":  s.SecondsLateOverride,
		"cached_due_date":        s.CachedDueDate,
		"custom_grade_status_id": s.CustomGradeStatusID,
		"extra_attempts":         s.ExtraAttempts,
	}
	if s.Attachments != nil && *s.Attachments != "" {
		var attachments []map[string]interface{}
		if err := json.Unmarshal([]byte(*s.Attachments), &attachments); err == nil {
			result["attachments"] = attachments
		}
	}
	return result
}

// studentSubmissionToJSON is what a student (or observer) sees. Until the
// submission is posted (posted_at set — immediately on grading for an
// automatic post policy, on "Post grades" for a manual one) every grade
// field is withheld, matching Canvas's post-policy contract.
func studentSubmissionToJSON(s *models.Submission) fiber.Map {
	m := submissionToJSON(s)
	if s.PostedAt == nil {
		for _, k := range []string{"score", "grade", "entered_score", "entered_grade", "points_deducted", "graded_at", "grader_id"} {
			m[k] = nil
		}
		// Excusing is part of the grade (Canvas hides it until posted too).
		m["excused"] = false
		if s.WorkflowState == models.SubmissionGraded {
			if s.SubmittedAt != nil {
				m["workflow_state"] = models.SubmissionSubmitted
			} else {
				m["workflow_state"] = models.SubmissionUnsubmitted
			}
		}
	}
	return m
}

// submissionJSONForCaller picks the staff or student representation.
func submissionJSONForCaller(c *fiber.Ctx, s *models.Submission) fiber.Map {
	if callerIsCourseStaff(c) {
		return submissionToJSON(s)
	}
	return studentSubmissionToJSON(s)
}

// peerReviewSubmissionJSON is the reviewer's view of a classmate's work:
// the content only, never the grade.
func peerReviewSubmissionJSON(s *models.Submission) fiber.Map {
	m := fiber.Map{
		"id":              s.ID,
		"assignment_id":   s.AssignmentID,
		"user_id":         s.UserID,
		"submission_type": s.SubmissionType,
		"body":            s.Body,
		"url":             s.URL,
		"submitted_at":    s.SubmittedAt,
		"attempt":         s.Attempt,
	}
	if s.Attachments != nil && *s.Attachments != "" {
		var attachments []map[string]interface{}
		if err := json.Unmarshal([]byte(*s.Attachments), &attachments); err == nil {
			m["attachments"] = attachments
		}
	}
	return m
}

func submissionCommentToJSON(sc *models.SubmissionComment) fiber.Map {
	return fiber.Map{
		"id":            sc.ID,
		"submission_id": sc.SubmissionID,
		"author_id":     sc.AuthorID,
		"comment":       sc.Comment,
		"draft":         sc.Draft,
		"created_at":    sc.CreatedAt,
		"updated_at":    sc.UpdatedAt,
	}
}

// callerIsCourseStaff reports whether the request's auth Locals mark the caller
// as a teacher/TA (enrollment_type, set by RequireEnrolled/RequireInstructor)
// or an admin — i.e. someone allowed to view other students' submissions.
// Students get false.
func callerIsCourseStaff(c *fiber.Ctx) bool {
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		return true
	}
	et, _ := c.Locals("enrollment_type").(string)
	return et == "TeacherEnrollment" || et == "TaEnrollment"
}

// requireAssignmentInCourse loads assignmentID under the caller's tenant
// and verifies it belongs to the URL's :course_id (F-013 parent-tie).
// Non-staff callers additionally cannot reach an unpublished assignment.
// On any miss it writes a 404 and returns wrote=true; the caller MUST
// `return nil` in that case ((result, wrote, err) convention).
func (h *SubmissionHandler) requireAssignmentInCourse(c *fiber.Ctx, assignmentID, courseID uint) (*models.Assignment, bool) {
	assignment, err := h.assignmentRepo.FindByID(c.Context(), assignmentID, callerAccountID(c))
	if err != nil || assignment == nil || assignment.CourseID != courseID ||
		assignment.WorkflowState == models.AssignmentDeleted ||
		(!callerIsCourseStaff(c) && !assignment.IsPublished()) {
		_ = responses.NotFound(c, "assignment")
		return nil, true
	}
	return assignment, false
}

var (
	errGradeTargetAssignment = errors.New("assignment not found")
	errGradeTargetStudent    = errors.New("user is not a student in this course")
)

// gradeTargetError validates that assignmentID is a (non-deleted)
// assignment of courseID in the caller's tenant and that userID is a
// student enrolled in that course. It never writes a response, so the
// bulk-grade loop can report per-entry errors.
func (h *SubmissionHandler) gradeTargetError(ctx context.Context, assignmentID, userID, courseID, accountID uint, cache map[uint]*models.Assignment) error {
	assignment, ok := cache[assignmentID]
	if !ok {
		a, err := h.assignmentRepo.FindByID(ctx, assignmentID, accountID)
		if err != nil {
			a = nil
		}
		assignment = a
		if cache != nil {
			cache[assignmentID] = a
		}
	}
	if assignment == nil || assignment.CourseID != courseID || assignment.WorkflowState == models.AssignmentDeleted {
		return errGradeTargetAssignment
	}
	if !h.submissionService.IsCourseStudent(ctx, courseID, userID, accountID) {
		return errGradeTargetStudent
	}
	return nil
}

// callerCanViewSubmission: the owner, course staff, or a linked observer.
func (h *SubmissionHandler) callerCanViewSubmission(c *fiber.Ctx, submissionUserID uint) bool {
	callerID, _ := c.Locals("user_id").(uint)
	if callerID != 0 && callerID == submissionUserID {
		return true
	}
	if callerIsCourseStaff(c) {
		return true
	}
	if h.observerService != nil {
		if ok, _ := h.observerService.IsObserverOf(c.Context(), callerID, submissionUserID); ok {
			return true
		}
	}
	return false
}

// callerIsAssignedPeerReviewer reports whether the caller has a peer
// review assigned for (assignmentID, revieweeID).
func (h *SubmissionHandler) callerIsAssignedPeerReviewer(c *fiber.Ctx, assignmentID, revieweeID uint) bool {
	if h.peerReviewService == nil {
		return false
	}
	callerID, _ := c.Locals("user_id").(uint)
	if callerID == 0 {
		return false
	}
	reviews, err := h.peerReviewService.ListByReviewer(c.Context(), assignmentID, callerID, callerAccountID(c))
	if err != nil {
		return false
	}
	for _, r := range reviews {
		if r.RevieweeID == revieweeID {
			return true
		}
	}
	return false
}

func (h *SubmissionHandler) ListCourseSubmissions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	// Filter to requesting user's submissions if user_id query param is present
	// This allows students (enrolled but not instructor) to fetch their own submissions
	// Observers can pass their observee's user_id to see that student's submissions
	requestingUserID, _ := c.Locals("user_id").(uint)
	isStaff := callerIsCourseStaff(c)
	filterUserID := uint(0)
	if uid := c.Query("user_id"); uid != "" {
		if uid == "self" {
			filterUserID = requestingUserID
		} else {
			if parsed, err := strconv.Atoi(uid); err == nil && parsed > 0 {
				targetUserID := uint(parsed)
				// If requesting someone else's submissions, verify observer link
				if targetUserID != requestingUserID && !isStaff {
					isObserver := false
					if h.observerService != nil {
						isObserver, _ = h.observerService.IsObserverOf(c.Context(), requestingUserID, targetUserID)
					}
					if !isObserver {
						return responses.BadRequest(c, "Not authorized to view this student's submissions")
					}
				}
				filterUserID = targetUserID
			}
		}
	}

	// Default-deny: a non-staff caller who did not scope the request to a
	// specific (self or observee) user_id is restricted to their own
	// submissions. Without this, omitting ?user_id returned every student's
	// submissions to any enrolled member (IDOR / FERPA leak).
	if filterUserID == 0 && !isStaff {
		filterUserID = requestingUserID
	}

	// One student's submissions: filter in SQL, then paginate. This used
	// to paginate the whole class first and filter afterwards, so with
	// the 100-row page cap a student saw only whichever of their rows
	// happened to fall in the first 100 of the course (B014).
	if filterUserID > 0 {
		all, err := h.submissionService.ListByUserAndCourse(c.Context(), filterUserID, uint(courseID))
		if err != nil {
			return responses.InternalError(c, "Could not fetch submissions")
		}
		sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
		total := len(all)
		start := (params.Page - 1) * params.PerPage
		if start > total {
			start = total
		}
		end := start + params.PerPage
		if end > total {
			end = total
		}
		responses.SetPaginationHeaders(c, int64(total), params.Page, params.PerPage)
		submissions := make([]fiber.Map, 0, end-start)
		for i := start; i < end; i++ {
			submissions = append(submissions, submissionJSONForCaller(c, &all[i]))
		}
		return c.JSON(submissions)
	}

	result, err := h.submissionService.BulkListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submissions")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
	result.Items = filterSubmissionsForSections(c, result.Items) // G107

	submissions := make([]fiber.Map, 0, len(result.Items))
	for i := range result.Items {
		submissions = append(submissions, submissionJSONForCaller(c, &result.Items[i]))
	}
	return c.JSON(submissions)
}

func (h *SubmissionHandler) ListSubmissions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	// B009: the assignment must be in the caller's tenant AND the URL's
	// course. Without the tie a teacher of course A could list every
	// submission of any assignment in any tenant.
	assignment, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return nil
	}

	callerID, _ := c.Locals("user_id").(uint)
	isStaff := callerIsCourseStaff(c)

	// Authorization: only course staff (teacher/TA/admin) may see the whole
	// class's submissions. A student sees only their own row; it is loaded
	// directly so pagination can't hide it.
	if !isStaff {
		submissions := make([]fiber.Map, 0, 1)
		if own, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), callerID, callerAccountID(c)); err == nil && own != nil {
			submissions = append(submissions, studentSubmissionToJSON(own))
		}
		responses.SetPaginationHeaders(c, int64(len(submissions)), 1, 1)
		return c.JSON(submissions)
	}

	params := middleware.GetPagination(c)
	result, err := h.submissionService.ListByAssignment(c.Context(), uint(assignmentID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submissions")
	}

	result.Items = filterSubmissionsForSections(c, result.Items) // G107
	// Anonymous grading: graders get the rows without user ids, addressed
	// by anonymous id (as SpeedGrader shows them).
	anonymized := h.staffAnonymized(c, uint(courseID), assignment)
	if anonymized {
		h.ensureAnonymousIDs(c, assignment, result.Items)
	}
	submissions := make([]fiber.Map, 0, len(result.Items))
	for i := range result.Items {
		if anonymized {
			submissions = append(submissions, anonymizedStaffSubmissionJSON(uint(courseID), &result.Items[i]))
			continue
		}
		submissions = append(submissions, submissionToJSON(&result.Items[i]))
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	// 13.5 PII audit — bulk-read semantics (per-row would emit N rows
	// per page load on a 200-student section, which floods the audit
	// surface for no real investigative win). One row per call with
	// student_id=0 + data_field="bulk_submission_list_read" preserves
	// "who looked at which assignment's submissions, when".
	if callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_submission_list_read", "submissions", uint(assignmentID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(submissions)
}

func (h *SubmissionHandler) GetSubmission(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	assignment, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return nil
	}

	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "submission")
	}
	// Anonymous grading: a grader may not address the student by user id
	// (that maps anonymous ids back to students); SpeedGrader uses the
	// anonymous_submissions routes instead. The student's own view is
	// unaffected.
	if callerID, _ := c.Locals("user_id").(uint); callerID != submission.UserID && h.staffAnonymized(c, uint(courseID), assignment) {
		return responses.NotFound(c, "submission")
	}

	// Authorization: only the submission owner, course staff (teacher/TA/admin),
	// or a linked observer may view another student's submission. An
	// assigned peer reviewer gets the work itself without the grade.
	// Existence-leak contract: 404 not 403.
	if !h.callerCanViewSubmission(c, submission.UserID) {
		if h.callerIsAssignedPeerReviewer(c, uint(assignmentID), submission.UserID) {
			return c.JSON(peerReviewSubmissionJSON(submission))
		}
		return responses.NotFound(c, "submission")
	}

	// 13.5 PII audit — single-student read; emit one row per access.
	if callerID, _ := c.Locals("user_id").(uint); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, submission.UserID, "read", "submission", "submissions", submission.ID, c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(submissionJSONForCaller(c, submission))
}

// submissionFileDownloadURL is the submission-scoped download route for a
// submitted file. Authorization there follows the submission (owner,
// staff, observer, assigned peer reviewer), not the file's personal
// context, so the grader can open a student's upload.
func submissionFileDownloadURL(courseID, assignmentID, userID, fileID uint) string {
	return fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d/files/%d/download", courseID, assignmentID, userID, fileID)
}

func (h *SubmissionHandler) CreateSubmission(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	// B111: only students submit. Observers (and staff) are "enrolled"
	// too, but a submission from them would be a stray graded row.
	if et, _ := c.Locals("enrollment_type").(string); et != "StudentEnrollment" {
		return responses.Forbidden(c, "Only students can submit to assignments")
	}

	var input struct {
		Submission struct {
			SubmissionType string `json:"submission_type"`
			Body           string `json:"body"`
			URL            string `json:"url"`
			FileIDs        []uint `json:"file_ids"`
		} `json:"submission"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	submission := &models.Submission{
		AssignmentID:   uint(assignmentID),
		UserID:         userID,
		SubmissionType: &input.Submission.SubmissionType,
	}
	if input.Submission.Body != "" {
		body := service.SanitizeHTML(input.Submission.Body)
		submission.Body = &body
	}

	if input.Submission.URL != "" {
		submission.URL = &input.Submission.URL
	}

	// Resolve file attachments. A student may only attach files they
	// uploaded themselves — otherwise any course file id (or a
	// classmate's upload) could be passed off as their own work.
	if len(input.Submission.FileIDs) > 0 {
		var attachments []map[string]interface{}
		for _, fid := range input.Submission.FileIDs {
			att, err := h.attachmentRepo.FindByID(c.Context(), fid, callerAccountID(c))
			if err != nil || att == nil || att.UserID != userID {
				return responses.BadRequest(c, fmt.Sprintf("File ID %d not found", fid))
			}
			attachments = append(attachments, map[string]interface{}{
				"id":           att.ID,
				"display_name": att.DisplayName,
				"filename":     att.Filename,
				"content_type": att.ContentType,
				"size":         att.Size,
				"url":          submissionFileDownloadURL(uint(courseID), uint(assignmentID), userID, att.ID),
			})
		}
		if len(attachments) > 0 {
			attJSON, _ := json.Marshal(attachments)
			attStr := string(attJSON)
			submission.Attachments = &attStr
		}
	}

	if err := h.submissionService.Submit(c.Context(), submission, uint(courseID), callerAccountID(c)); err != nil {
		// F-047: distinct status codes for the lock/unlock windows so
		// the client UI can show the right "not yet" vs "too late"
		// message. 409 Conflict matches the state-machine semantics.
		switch err {
		case service.ErrSubmissionLocked, service.ErrSubmissionNotYetUnlocked, service.ErrAttemptsExhausted:
			return responses.Error(c, fiber.StatusConflict, err.Error())
		case service.ErrAssignmentUnavailable:
			return responses.NotFound(c, "assignment")
		}
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(studentSubmissionToJSON(submission))
}

// notifyGradePosted queues the "grade posted" notification for a
// submission whose grade the student can now see.
func (h *SubmissionHandler) notifyGradePosted(ctx context.Context, assignment *models.Assignment, sub *models.Submission) {
	service.NotifyGradePosted(ctx, h.notificationDeliveryService, assignment, sub)
}

// recordOutcomeResults upserts the learning-outcome results of a graded,
// POSTED submission for every outcome aligned to the assignment. An
// unposted grade must not surface through the outcome/mastery views.
func (h *SubmissionHandler) recordOutcomeResults(ctx context.Context, assignment *models.Assignment, sub *models.Submission, accountID uint) {
	service.RecordAssignmentOutcomeResults(ctx, h.outcomeAlignmentRepo, h.outcomeService, assignment, sub, accountID)
}

func (h *SubmissionHandler) UpdateSubmission(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	submission, wrote, err := h.updateSubmissionFor(c, uint(courseID), uint(assignmentID), uint(userID))
	if wrote || err != nil {
		return err
	}
	return c.JSON(submissionToJSON(submission))
}

// submissionUpdateInput is the grader's PUT body (Canvas submission[...]).
type submissionUpdateInput struct {
	Submission struct {
		// Pointer so an explicit "" (clear the grade) is
		// distinguishable from "not provided".
		PostedGrade         *string `json:"posted_grade"`
		Excused             *bool   `json:"excused"`
		CustomGradeStatusID *uint   `json:"custom_grade_status_id"`
		LatePolicyStatus    *string `json:"late_policy_status"`
		ExtraAttempts       *int    `json:"extra_attempts"`
	} `json:"submission"`
}

// updateSubmissionFor grades / excuses / sets the status of userID's
// submission. Shared by the user-addressed route and the anonymous
// SpeedGrader route. (result, wrote, err): on wrote=true a response was
// written and the caller returns err as-is.
func (h *SubmissionHandler) updateSubmissionFor(c *fiber.Ctx, courseID, assignmentID, userID uint) (*models.Submission, bool, error) {
	graderID, err := getUserID(c)
	if err != nil {
		return nil, true, err
	}

	var input submissionUpdateInput
	if err := c.BodyParser(&input); err != nil {
		return nil, true, responses.BadRequest(c, "Invalid input")
	}
	in := input.Submission
	status := service.SubmissionStatusUpdate{
		CustomGradeStatusID: in.CustomGradeStatusID,
		LatePolicyStatus:    in.LatePolicyStatus,
		ExtraAttempts:       in.ExtraAttempts,
		GraderID:            graderID,
	}
	if in.PostedGrade == nil && in.Excused == nil && status.Empty() {
		return nil, true, responses.BadRequest(c, "No valid update fields provided")
	}

	// B015 / F-013: the assignment must belong to :course_id (and the
	// caller's tenant), and the graded user must be a student of that
	// course. Without this a teacher of course A could grade course B's
	// assignments or mint graded submissions for arbitrary user ids.
	acct := callerAccountID(c)
	if err := h.gradeTargetError(c.Context(), assignmentID, userID, courseID, acct, nil); err != nil {
		if err == errGradeTargetStudent {
			return nil, true, responses.NotFound(c, "user")
		}
		return nil, true, responses.NotFound(c, "assignment")
	}
	assignment, aErr := h.assignmentRepo.FindByID(c.Context(), assignmentID, acct)
	if aErr != nil {
		return nil, true, responses.NotFound(c, "assignment")
	}

	var submission *models.Submission

	// Grader-set statuses (custom status, late/missing/extended, extra
	// attempts) are applied first so a combined update returns them.
	if !status.Empty() {
		if h.gradingUI == nil {
			return nil, true, responses.BadRequest(c, "Submission statuses are not available")
		}
		courseAcct, cErr := h.gradingUI.CourseAccountID(c.Context(), courseID, acct)
		if cErr != nil {
			return nil, true, responses.NotFound(c, "assignment")
		}
		sub, sErr := h.gradingUI.UpdateSubmissionStatus(c.Context(), assignment, userID, courseAcct, status)
		switch {
		case sErr == service.ErrCustomStatusNotFound:
			return nil, true, responses.NotFound(c, "custom grade status")
		case errors.Is(sErr, service.ErrModerationInProgress):
			return nil, true, responses.Error(c, fiber.StatusConflict, sErr.Error())
		case sErr != nil:
			return nil, true, responses.BadRequest(c, sErr.Error())
		}
		submission = sub
	}

	// Excusing takes precedence over a grade (Canvas: an excused
	// submission has no grade to post).
	if in.Excused != nil {
		excused := *in.Excused
		sub, exErr := h.submissionService.SetExcused(c.Context(), assignmentID, userID, graderID, acct, excused)
		switch {
		case exErr == service.ErrGradeCrossTenant:
			return nil, true, responses.NotFound(c, "assignment")
		case exErr == service.ErrModerationInProgress:
			return nil, true, responses.Error(c, fiber.StatusConflict, exErr.Error())
		case exErr == service.ErrSubmissionNotFound:
			// Un-excusing a student with no submission row: nothing to
			// change. Fall through to grading if a grade was also sent.
			if in.PostedGrade == nil && submission == nil {
				return nil, true, responses.NotFound(c, "submission")
			}
		case exErr != nil:
			return nil, true, responses.BadRequest(c, exErr.Error())
		default:
			submission = sub
		}
		if excused && submission != nil {
			h.notifyGradePosted(c.Context(), assignment, submission)
			return submission, false, nil
		}
	}

	if in.PostedGrade != nil {
		// F-016: pass caller's tenant so cross-tenant grading attempts
		// (teacher in tenant A → assignment in tenant B) return 404.
		graded, gErr := h.submissionService.Grade(c.Context(), assignmentID, userID, graderID, acct, *in.PostedGrade)
		if gErr != nil {
			switch gErr {
			case service.ErrGradeCrossTenant:
				return nil, true, responses.NotFound(c, "assignment")
			case service.ErrModerationInProgress:
				return nil, true, responses.Error(c, fiber.StatusConflict, gErr.Error())
			}
			return nil, true, responses.BadRequest(c, gErr.Error())
		}
		submission = graded

		// B012: only notify when the student can actually see the grade
		// (automatic post policy, or an already-posted submission).
		h.notifyGradePosted(c.Context(), assignment, submission)

		// Outcome results are student-visible (mastery views), so they
		// are recorded only once the grade itself is visible; a manual
		// post policy records them when grades are posted.
		h.recordOutcomeResults(c.Context(), assignment, submission, acct)
	}

	if submission == nil {
		return nil, true, responses.NotFound(c, "submission")
	}
	return submission, false, nil
}

// loadCommentableSubmission resolves the submission addressed by the
// comment routes after the course parent-tie. Returns wrote=true after
// writing a response on any miss.
func (h *SubmissionHandler) loadCommentableSubmission(c *fiber.Ctx) (*models.Assignment, *models.Submission, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, nil, true
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid assignment ID")
		return nil, nil, true
	}
	userID, err := c.ParamsInt("user_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid user ID")
		return nil, nil, true
	}
	assignment, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return nil, nil, true
	}
	submission, err := h.submissionService.GetByAssignmentAndUser(c.Context(), uint(assignmentID), uint(userID), callerAccountID(c))
	if err != nil {
		_ = responses.NotFound(c, "submission")
		return nil, nil, true
	}
	return assignment, submission, false
}

// commentInput is the comment POST body (Canvas comment[...]).
type commentInput struct {
	Comment struct {
		TextComment  string `json:"text_comment"`
		FileIDs      []uint `json:"file_ids"`
		GroupComment bool   `json:"group_comment"`
		Draft        bool   `json:"draft"`
	} `json:"comment"`
}

// commentPathKey is the URL segment that addresses a submission in a
// comment-file download link: "submissions/<user_id>" normally, or
// "anonymous_submissions/<anonymous_id>" under anonymous grading so the
// link never carries the student's id.
func commentPathKey(sub *models.Submission, anonymize bool) string {
	if anonymize && sub.AnonymousID != nil {
		return "anonymous_submissions/" + *sub.AnonymousID
	}
	return fmt.Sprintf("submissions/%d", sub.UserID)
}

// commentJSON renders a comment with download links for its files.
func commentJSON(courseID, assignmentID uint, pathKey string, sc *models.SubmissionComment) fiber.Map {
	j := submissionCommentToJSON(sc)
	j["group_comment_id"] = sc.GroupCommentID
	j["attempt"] = sc.Attempt
	files := make([]map[string]interface{}, 0)
	if sc.Attachments != nil && *sc.Attachments != "" {
		_ = json.Unmarshal([]byte(*sc.Attachments), &files)
		for _, f := range files {
			if id, ok := f["id"].(float64); ok {
				f["url"] = fmt.Sprintf("/api/v1/courses/%d/assignments/%d/%s/comments/%d/files/%d/download", courseID, assignmentID, pathKey, sc.ID, uint(id))
			}
		}
	}
	j["attachments"] = files
	return j
}

// commentVisibleTo decides whether callerID may read sc on submission.
// Drafts are private to their author. Students and observers see only
// the student's own comments until the submission is posted. On a
// moderated assignment whose grades are unpublished, graders see other
// graders' comments only when the assignment allows it (the moderator
// always does).
func (h *SubmissionHandler) commentVisibleTo(c *fiber.Ctx, assignment *models.Assignment, submission *models.Submission, sc *models.SubmissionComment, callerID uint, isStaff bool) bool {
	if sc.Draft && sc.AuthorID != callerID {
		return false
	}
	if !isStaff {
		return submission.PostedAt != nil || sc.AuthorID == submission.UserID
	}
	if assignment != nil && assignment.ModeratedGrading && !assignment.GradesPublished() &&
		!assignment.GraderCommentsVisibleToGraders && sc.AuthorID != callerID && sc.AuthorID != submission.UserID {
		isAdmin, _ := c.Locals("is_admin").(bool)
		return service.IsModerator(assignment, callerID, isAdmin)
	}
	return true
}

func (h *SubmissionHandler) CreateSubmissionComment(c *fiber.Ctx) error {
	if _, err := getUserID(c); err != nil {
		return err
	}
	assignment, submission, wrote := h.loadCommentableSubmission(c)
	if wrote {
		return nil
	}
	j, wrote, err := h.createCommentFor(c, assignment, submission, false)
	if wrote || err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(j)
}

// createCommentFor adds a comment (with optional files, as a draft, or
// to every member of a group) to submission. (result, wrote, err).
func (h *SubmissionHandler) createCommentFor(c *fiber.Ctx, assignment *models.Assignment, submission *models.Submission, anonymize bool) (fiber.Map, bool, error) {
	authorID, err := getUserID(c)
	if err != nil {
		return nil, true, err
	}

	var input commentInput
	if err := c.BodyParser(&input); err != nil {
		return nil, true, responses.BadRequest(c, "Invalid input")
	}
	in := input.Comment
	if strings.TrimSpace(in.TextComment) == "" && len(in.FileIDs) == 0 {
		return nil, true, responses.BadRequest(c, "Comment text is required")
	}
	if len(in.FileIDs) > 20 {
		return nil, true, responses.BadRequest(c, "A comment can carry at most 20 files")
	}

	// B016: only the submission owner and course staff may comment.
	// Classmates (and observers) get the same 404 as a missing row.
	isStaff := callerIsCourseStaff(c)
	if !isStaff && authorID != submission.UserID {
		return nil, true, responses.NotFound(c, "submission")
	}
	// Drafts are a grader tool (Canvas); a student comment is always sent.
	draft := in.Draft && isStaff

	// Attached files (documents or recorded audio/video) must be the
	// author's own uploads.
	var attachmentsJSON *string
	if len(in.FileIDs) > 0 {
		files := make([]map[string]interface{}, 0, len(in.FileIDs))
		for _, fid := range in.FileIDs {
			att, aErr := h.attachmentRepo.FindByID(c.Context(), fid, callerAccountID(c))
			if aErr != nil || att == nil || att.UserID != authorID {
				return nil, true, responses.BadRequest(c, fmt.Sprintf("File ID %d not found", fid))
			}
			files = append(files, map[string]interface{}{
				"id":           att.ID,
				"display_name": att.DisplayName,
				"filename":     att.Filename,
				"content_type": att.ContentType,
				"size":         att.Size,
			})
		}
		raw, _ := json.Marshal(files)
		s := string(raw)
		attachmentsJSON = &s
	}

	targets := []*models.Submission{submission}
	var groupCommentID *string
	if in.GroupComment && assignment.IsGroupAssignment() && h.gradingUI != nil {
		if subs, gErr := h.gradingUI.GroupSubmissions(c.Context(), assignment, submission.UserID); gErr == nil && len(subs) > 1 {
			targets = subs
			gid := uuid.NewString()
			groupCommentID = &gid
		}
	}

	var primary *models.SubmissionComment
	for _, target := range targets {
		attempt := target.Attempt
		comment := &models.SubmissionComment{
			SubmissionID:   target.ID,
			AuthorID:       authorID,
			Comment:        in.TextComment,
			Draft:          draft,
			Attachments:    attachmentsJSON,
			GroupCommentID: groupCommentID,
			Attempt:        &attempt,
		}
		if err := h.commentRepo.Create(c.Context(), comment); err != nil {
			return nil, true, responses.InternalError(c, "Could not create comment")
		}
		if primary == nil {
			primary = comment
		}
		// Canvas: on an automatically-posted assignment an instructor
		// comment posts the submission, so the student can read the
		// feedback. Not while a moderated assignment is unpublished.
		if isStaff && !draft && !assignment.UsesManualPostPolicy() && assignment.GradesPublished() {
			_ = h.submissionService.MarkPosted(c.Context(), target)
		}
	}

	courseID, _ := c.ParamsInt("course_id")
	j := commentJSON(uint(courseID), assignment.ID, commentPathKey(submission, anonymize), primary)
	authorName := ""
	if author, err := h.userRepo.FindByID(c.Context(), authorID, callerAccountID(c)); err == nil {
		j["author_name"] = author.Name
		authorName = author.Name
	}
	// G091: tell each student who received the comment (drafts wait
	// until they are published).
	if !draft {
		for _, target := range targets {
			h.notifySubmissionComment(c.Context(), assignment, target, authorID, authorName, in.TextComment)
		}
	}
	return j, false, nil
}

func (h *SubmissionHandler) ListSubmissionComments(c *fiber.Ctx) error {
	assignment, submission, wrote := h.loadCommentableSubmission(c)
	if wrote {
		return nil
	}

	// B016: owner, course staff or a linked observer only.
	if !h.callerCanViewSubmission(c, submission.UserID) {
		return responses.NotFound(c, "submission")
	}
	result, wrote, err := h.listCommentsFor(c, assignment, submission, false)
	if wrote || err != nil {
		return err
	}
	return c.JSON(result)
}

// listCommentsFor renders the comments callerID may see on submission.
// Under anonymize the student's own comments carry no name or id.
func (h *SubmissionHandler) listCommentsFor(c *fiber.Ctx, assignment *models.Assignment, submission *models.Submission, anonymize bool) ([]fiber.Map, bool, error) {
	isStaff := callerIsCourseStaff(c)
	callerID, _ := c.Locals("user_id").(uint)

	comments, err := h.commentRepo.ListBySubmissionID(c.Context(), submission.ID, callerAccountID(c))
	if err != nil {
		return nil, true, responses.InternalError(c, "Could not fetch comments")
	}

	visible := make([]models.SubmissionComment, 0, len(comments))
	for i := range comments {
		if h.commentVisibleTo(c, assignment, submission, &comments[i], callerID, isStaff) {
			visible = append(visible, comments[i])
		}
	}

	// Build author name cache to avoid N+1
	authorNames := make(map[uint]string)
	for _, sc := range visible {
		if _, ok := authorNames[sc.AuthorID]; !ok {
			if author, err := h.userRepo.FindByID(c.Context(), sc.AuthorID, callerAccountID(c)); err == nil {
				authorNames[sc.AuthorID] = author.Name
			}
		}
	}

	// Moderated grading: while grader identities are hidden from this
	// grader, other graders' comments carry only their grader label.
	var graderLabels map[uint]string
	isAdmin, _ := c.Locals("is_admin").(bool)
	hideGraders := isStaff && !service.CanViewOtherGraderIdentities(assignment, callerID, isAdmin)
	if hideGraders && h.gradingUI != nil {
		graderLabels = h.gradingUI.GraderLabels(c.Context(), assignment.ID)
	}

	courseID, _ := c.ParamsInt("course_id")
	pathKey := commentPathKey(submission, anonymize)
	result := make([]fiber.Map, len(visible))
	for i := range visible {
		j := commentJSON(uint(courseID), submission.AssignmentID, pathKey, &visible[i])
		author := visible[i].AuthorID
		if anonymize && author == submission.UserID {
			j["author_id"] = nil
			j["author_name"] = "Student"
		} else if hideGraders && author != submission.UserID && author != callerID {
			j["author_id"] = nil
			j["author_name"] = "Grader"
			if label := graderLabels[author]; label != "" {
				j["author_name"] = label
			}
		} else if name, ok := authorNames[visible[i].AuthorID]; ok {
			j["author_name"] = name
		}
		result[i] = j
	}

	// 13.5 PII audit — student-keyed read of comments on a single
	// submission. Emit one row per access with the submission's
	// owning student as the subject.
	if callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, submission.UserID, "read", "submission_comments", "submission_comments", submission.ID, c.IP(), c.Get("User-Agent"))
	}

	return result, false, nil
}

func (h *SubmissionHandler) BulkGrade(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	graderID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		GradeData []struct {
			AssignmentID uint   `json:"assignment_id"`
			UserID       uint   `json:"user_id"`
			PostedGrade  string `json:"posted_grade"`
			// Score is a score already in points (gradebook CSV import,
			// curve): written as-is, never read as a typed grade (a 7 on a
			// percent assignment stays 7 points). Takes precedence over
			// posted_grade.
			Score *float64 `json:"score"`
		} `json:"grade_data"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.GradeData) == 0 {
		return responses.BadRequest(c, "grade_data must not be empty")
	}
	if len(input.GradeData) > 10000 {
		return responses.BadRequest(c, "grade_data exceeds maximum of 10000 entries")
	}

	type gradeResult struct {
		AssignmentID uint     `json:"assignment_id"`
		UserID       uint     `json:"user_id"`
		Score        *float64 `json:"score,omitempty"`
		Grade        *string  `json:"grade,omitempty"`
		EnteredScore *float64 `json:"entered_score,omitempty"`
		Error        string   `json:"error,omitempty"`
	}

	results := make([]gradeResult, 0, len(input.GradeData))

	callerAcct := callerAccountID(c)
	assignmentCache := make(map[uint]*models.Assignment)
	for _, entry := range input.GradeData {
		// B015 / F-013 / F-016: each entry's assignment must be in this
		// course (and tenant) and the user a student of this course.
		if tErr := h.gradeTargetError(c.Context(), entry.AssignmentID, entry.UserID, uint(courseID), callerAcct, assignmentCache); tErr != nil {
			results = append(results, gradeResult{AssignmentID: entry.AssignmentID, UserID: entry.UserID, Error: tErr.Error()})
			continue
		}
		if !sectionVisibleUser(c, entry.UserID) { // G107: outside the grader's sections
			results = append(results, gradeResult{AssignmentID: entry.AssignmentID, UserID: entry.UserID, Error: "user not found"})
			continue
		}
		var sub *models.Submission
		var err error
		if entry.Score != nil {
			// GradeScore notifies the student itself when the visible
			// grade changed (SetGradeNotifier).
			sub, err = h.submissionService.GradeScore(c.Context(), entry.AssignmentID, entry.UserID, graderID, callerAcct, *entry.Score)
		} else {
			sub, err = h.submissionService.Grade(c.Context(), entry.AssignmentID, entry.UserID, graderID, callerAcct, entry.PostedGrade)
		}
		if err != nil {
			errMsg := err.Error()
			if err == service.ErrGradeCrossTenant {
				errMsg = "assignment not found"
			}
			results = append(results, gradeResult{
				AssignmentID: entry.AssignmentID,
				UserID:       entry.UserID,
				Error:        errMsg,
			})
			continue
		}
		if entry.Score == nil {
			h.notifyGradePosted(c.Context(), assignmentCache[entry.AssignmentID], sub)
		}
		// Curve, CSV import and bulk entry post at once on an automatic
		// post policy, so they record outcome results like SpeedGrader.
		h.recordOutcomeResults(c.Context(), assignmentCache[entry.AssignmentID], sub, callerAcct)
		results = append(results, gradeResult{
			AssignmentID: entry.AssignmentID,
			UserID:       entry.UserID,
			Score:        sub.Score,
			Grade:        sub.Grade,
			EnteredScore: sub.EnteredScore,
		})
	}

	return c.JSON(fiber.Map{"results": results})
}

// PostGrades sets posted_at on all graded submissions for an assignment,
// making grades visible to students, and notifies the students whose
// grades were just released.
func (h *SubmissionHandler) PostGrades(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	// B011: tie the assignment to :course_id and the caller's tenant.
	assignment, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return nil
	}
	// A moderated assignment has nothing to post until the moderator
	// publishes its grades (Canvas parity).
	if !assignment.GradesPublished() {
		return responses.Error(c, fiber.StatusConflict, "Grades for this moderated assignment have not been published yet")
	}

	// G018: "Graded" (only graded / excused work) or "Everyone", and
	// optionally only some sections, through the grading-policy service.
	if h.gradingPolicy != nil {
		opts, ok := parsePostOptions(c)
		if !ok {
			return responses.BadRequest(c, "Invalid input")
		}
		// G107: a section-limited grader posts / hides only their own
		// sections (no section_ids must not mean "every section").
		if opts.SectionIDs, ok = clampPostSections(c, opts.SectionIDs); !ok {
			return responses.NotFound(c, "section")
		}
		course, cErr := h.gradingPolicy.Repo().FindCourse(c.Context(), assignment.CourseID)
		if cErr != nil {
			return responses.NotFound(c, "course")
		}
		released, posted, pErr := h.gradingPolicy.PostGradesWithPosted(c.Context(), course, assignment, opts)
		if pErr != nil {
			return responses.InternalError(c, "Could not post grades")
		}
		postedIDs := make([]uint, 0, len(released))
		for i := range released {
			h.notifyGradePosted(c.Context(), assignment, &released[i])
			h.recordOutcomeResults(c.Context(), assignment, &released[i], callerAccountID(c))
			postedIDs = append(postedIDs, released[i].ID)
		}
		// Comments held back while the submissions were hidden become
		// visible now: tell the students (G091).
		for i := range posted {
			h.notifyHeldComments(c.Context(), assignment, &posted[i])
		}
		h.submissionService.FirePosted(postedIDs...)
		return c.JSON(fiber.Map{"posted": true, "posted_at": time.Now(), "posted_count": len(released)})
	}

	// Without the grading-policy service posting is all-or-nothing, which
	// a section-limited grader may not do (G107).
	if _, limited := middleware.VisibleSectionIDs(c); limited {
		return responses.NotFound(c, "section")
	}

	// Collect the submissions this call newly posts so their students
	// can be notified once the update lands.
	var newlyPosted, unposted []models.Submission
	if existing, lErr := h.submissionService.ListByAssignment(c.Context(), assignment.ID, repository.PaginationParams{Page: 1, PerPage: 10000}); lErr == nil {
		for _, s := range existing.Items {
			if s.PostedAt == nil {
				unposted = append(unposted, s)
				if s.Score != nil || s.Excused {
					newlyPosted = append(newlyPosted, s)
				}
			}
		}
	}

	now := time.Now()
	if err := h.submissionService.PostGradesByAssignment(c.Context(), assignment.ID, &now); err != nil {
		return responses.InternalError(c, "Could not post grades")
	}

	postedIDs := make([]uint, 0, len(newlyPosted))
	for i := range newlyPosted {
		newlyPosted[i].PostedAt = &now
		h.notifyGradePosted(c.Context(), assignment, &newlyPosted[i])
		h.recordOutcomeResults(c.Context(), assignment, &newlyPosted[i], callerAccountID(c))
		postedIDs = append(postedIDs, newlyPosted[i].ID)
	}
	for i := range unposted {
		unposted[i].PostedAt = &now
		h.notifyHeldComments(c.Context(), assignment, &unposted[i])
	}
	// Mastery paths, gamification, etc. react to a grade when the student
	// can see it: for a manual post policy, that is now.
	h.submissionService.FirePosted(postedIDs...)

	return c.JSON(fiber.Map{"posted": true, "posted_at": now})
}

// HideGrades clears posted_at on all submissions for an assignment,
// hiding grades from students.
func (h *SubmissionHandler) HideGrades(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	// B011: tie the assignment to :course_id and the caller's tenant.
	assignment, wrote := h.requireAssignmentInCourse(c, uint(assignmentID), uint(courseID))
	if wrote {
		return nil
	}

	if h.gradingPolicy != nil {
		opts, ok := parsePostOptions(c)
		if !ok {
			return responses.BadRequest(c, "Invalid input")
		}
		// G107: a section-limited grader posts / hides only their own
		// sections (no section_ids must not mean "every section").
		if opts.SectionIDs, ok = clampPostSections(c, opts.SectionIDs); !ok {
			return responses.NotFound(c, "section")
		}
		course, cErr := h.gradingPolicy.Repo().FindCourse(c.Context(), assignment.CourseID)
		if cErr != nil {
			return responses.NotFound(c, "course")
		}
		// Hiding switches the assignment to manual posting (Canvas), so
		// later grades and comments stay hidden until grades are posted.
		if err := h.gradingPolicy.HideGrades(c.Context(), course, assignment, opts); err != nil {
			return responses.InternalError(c, "Could not hide grades")
		}
		return c.JSON(fiber.Map{"hidden": true, "post_policy": assignment.PostPolicy})
	}

	if _, limited := middleware.VisibleSectionIDs(c); limited { // G107: see PostGrades
		return responses.NotFound(c, "section")
	}
	if !assignment.UsesManualPostPolicy() {
		assignment.PostPolicy = "manual"
		if err := h.assignmentRepo.Update(c.Context(), assignment); err != nil {
			return responses.InternalError(c, "Could not hide grades")
		}
	}
	if err := h.submissionService.PostGradesByAssignment(c.Context(), assignment.ID, nil); err != nil {
		return responses.InternalError(c, "Could not hide grades")
	}

	return c.JSON(fiber.Map{"hidden": true, "post_policy": assignment.PostPolicy})
}

// parsePostOptions reads the optional post / hide body:
// {"graded_only": true, "section_ids": [1, 2]}. An empty body is
// "everyone in every section".
func parsePostOptions(c *fiber.Ctx) (service.PostOptions, bool) {
	var opts service.PostOptions
	if len(c.Body()) == 0 {
		return opts, true
	}
	var input struct {
		GradedOnly bool   `json:"graded_only"`
		SectionIDs []uint `json:"section_ids"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return opts, false
	}
	opts.GradedOnly = input.GradedOnly
	opts.SectionIDs = dedupeUints(input.SectionIDs)
	return opts, true
}
