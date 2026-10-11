package handlers

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AssignmentHandler struct {
	assignmentService *service.AssignmentService
	// gradingPolicy (optional): course post-policy default, grading
	// scheme checks and late/missing refresh when dates change.
	gradingPolicy *service.GradingPolicyService
}

// SetGradingPolicyService wires the grading-policy collaborator.
func (h *AssignmentHandler) SetGradingPolicyService(p *service.GradingPolicyService) { h.gradingPolicy = p }

func NewAssignmentHandler(assignmentService *service.AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{assignmentService: assignmentService}
}

// studentDates returns the calling student's effective dates per
// assignment of the course (G019): an assignment that is only visible to
// overrides is absent unless an override applies to the student. ok is
// false (no filtering) for staff, observers and other roles, or when the
// grading-policy service is not wired.
func (h *AssignmentHandler) studentDates(c *fiber.Ctx, courseID uint) (map[uint]service.EffectiveDueDate, bool) {
	if h.gradingPolicy == nil || callerIsCourseStaff(c) {
		return nil, false
	}
	if et, _ := c.Locals("enrollment_type").(string); et != "StudentEnrollment" {
		return nil, false
	}
	callerID, _ := c.Locals("user_id").(uint)
	course, err := h.gradingPolicy.Repo().FindCourse(c.Context(), courseID)
	if err != nil || course == nil || course.AccountID != callerAccountID(c) {
		return nil, false
	}
	dates, err := h.gradingPolicy.StudentAssignmentDates(c.Context(), course, callerID)
	if err != nil {
		return nil, false
	}
	return dates, true
}

// withStudentDates is the assignment JSON with the student's own
// effective dates (override and accommodation applied), as Canvas returns
// due_at / unlock_at / lock_at for the current user.
func withStudentDates(a *models.Assignment, d service.EffectiveDueDate) fiber.Map {
	m := assignmentToJSON(a)
	m["due_at"], m["unlock_at"], m["lock_at"] = d.DueAt, d.UnlockAt, d.LockAt
	return m
}

func assignmentToJSON(a *models.Assignment) fiber.Map {
	return fiber.Map{
		"id":                   a.ID,
		"course_id":            a.CourseID,
		"assignment_group_id":  a.AssignmentGroupID,
		"name":                 a.Name,
		"description":          a.Description,
		"due_at":               a.DueAt,
		"unlock_at":            a.UnlockAt,
		"lock_at":              a.LockAt,
		"points_possible":      a.PointsPossible,
		"grading_type":         a.GradingType,
		"submission_types":     strings.Split(a.SubmissionTypes, ","),
		"position":             a.Position,
		"workflow_state":       a.WorkflowState,
		"published":            a.Published,
		"anonymous_grading":    a.AnonymousGrading,
		"post_policy":          a.PostPolicy,
		"peer_reviews_enabled": a.PeerReviewsEnabled,
		"peer_review_count":    a.PeerReviewCount,
		"group_category_id":    a.GroupCategoryID,
		"is_group_assignment":  a.GroupCategoryID != nil && *a.GroupCategoryID > 0,
		"only_visible_to_overrides": a.OnlyVisibleToOverrides,
		"grading_standard_id":       a.GradingStandardID,
		"created_at":           a.CreatedAt,
		"updated_at":           a.UpdatedAt,
	}
}

func (h *AssignmentHandler) ListAssignments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	// B116: students and observers only ever see published assignments
	// (Canvas hides unpublished drafts — title, description and all).
	// G080: students also lose unreleased Mastery Paths assignments.
	result, err := h.assignmentService.ListVisibleByCourseFor(c.Context(), uint(courseID), params, callerIsCourseStaff(c), callerUserID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignments")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	// G019: a student does not see assignments that are not assigned to
	// them, and sees their own dates on the ones that are.
	dates, filtered := h.studentDates(c, uint(courseID))
	assignments := make([]fiber.Map, 0, len(result.Items))
	for i := range result.Items {
		a := &result.Items[i]
		if !filtered {
			assignments = append(assignments, assignmentToJSON(a))
			continue
		}
		if d, ok := dates[a.ID]; ok {
			assignments = append(assignments, withStudentDates(a, d))
		}
	}

	return c.JSON(assignments)
}

func (h *AssignmentHandler) GetAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	assignment, err := h.assignmentService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "assignment")
	}
	// F-013 parent-tie, deleted rows are gone, and (B116) a draft is
	// invisible to students/observers — all the same 404.
	if assignment.CourseID != uint(courseID) || assignment.WorkflowState == models.AssignmentDeleted ||
		(!callerIsCourseStaff(c) && !assignment.IsPublished()) {
		return responses.NotFound(c, "assignment")
	}
	// G019: an assignment not assigned to the calling student is a 404.
	if dates, filtered := h.studentDates(c, uint(courseID)); filtered {
		d, ok := dates[assignment.ID]
		if !ok {
			return responses.NotFound(c, "assignment")
		}
		return c.JSON(withStudentDates(assignment, d))
	}

	return c.JSON(assignmentToJSON(assignment))
}

func (h *AssignmentHandler) CreateAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Assignment struct {
			Name               string     `json:"name"`
			Description        string     `json:"description"`
			DueAt              *time.Time `json:"due_at"`
			UnlockAt           *time.Time `json:"unlock_at"`
			LockAt             *time.Time `json:"lock_at"`
			PointsPossible     *float64   `json:"points_possible"`
			GradingType        string     `json:"grading_type"`
			SubmissionTypes    []string   `json:"submission_types"`
			Position           int        `json:"position"`
			Published          bool       `json:"published"`
			AssignmentGroupID  *uint      `json:"assignment_group_id"`
			AnonymousGrading   bool       `json:"anonymous_grading"`
			PostPolicy         string     `json:"post_policy"`
			PeerReviewsEnabled bool       `json:"peer_reviews_enabled"`
			PeerReviewCount    int        `json:"peer_review_count"`
			GroupCategoryID    *uint      `json:"group_category_id"`
			OnlyVisibleToOverrides bool  `json:"only_visible_to_overrides"`
			GradingStandardID      *uint `json:"grading_standard_id"`
		} `json:"assignment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if strings.TrimSpace(input.Assignment.Name) == "" {
		return responses.BadRequest(c, "Assignment name is required")
	}

	state := models.AssignmentUnpublished
	if input.Assignment.Published {
		state = models.AssignmentPublished
	}

	submissionTypes := strings.Join(input.Assignment.SubmissionTypes, ",")

	postPolicy := input.Assignment.PostPolicy
	if postPolicy == "" {
		// G018: new assignments follow the course post policy;
		// anonymous grading always posts manually (Canvas).
		postPolicy = "automatic"
		if input.Assignment.AnonymousGrading || (h.gradingPolicy != nil && h.gradingPolicy.CoursePostManually(c.Context(), uint(courseID))) {
			postPolicy = "manual"
		}
	}
	if msg := h.validateGradingFields(c, uint(courseID), input.Assignment.GradingType, postPolicy, input.Assignment.GradingStandardID); msg != "" {
		return responses.BadRequest(c, msg)
	}
	if input.Assignment.AnonymousGrading {
		// Canvas: anonymous assignments always post manually, whatever
		// post_policy was sent, so no grade shows before the reveal.
		postPolicy = "manual"
	}

	assignment := &models.Assignment{
		CourseID:           uint(courseID),
		AssignmentGroupID:  input.Assignment.AssignmentGroupID,
		Name:               input.Assignment.Name,
		Description:        service.SanitizeHTML(input.Assignment.Description),
		DueAt:              input.Assignment.DueAt,
		UnlockAt:           input.Assignment.UnlockAt,
		LockAt:             input.Assignment.LockAt,
		PointsPossible:     input.Assignment.PointsPossible,
		GradingType:        input.Assignment.GradingType,
		SubmissionTypes:    submissionTypes,
		Position:           input.Assignment.Position,
		Published:          input.Assignment.Published,
		WorkflowState:      state,
		AnonymousGrading:   input.Assignment.AnonymousGrading,
		PostPolicy:         postPolicy,
		PeerReviewsEnabled: input.Assignment.PeerReviewsEnabled,
		PeerReviewCount:    input.Assignment.PeerReviewCount,
		GroupCategoryID:    input.Assignment.GroupCategoryID,
		OnlyVisibleToOverrides: input.Assignment.OnlyVisibleToOverrides,
		GradingStandardID:      nonZeroUint(input.Assignment.GradingStandardID),
	}

	if assignment.GradingType == "" {
		assignment.GradingType = "points"
	}
	if assignment.SubmissionTypes == "" {
		assignment.SubmissionTypes = "online_text_entry"
	}

	if err := h.assignmentService.Create(c.Context(), assignment); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(assignmentToJSON(assignment))
}

func (h *AssignmentHandler) UpdateAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	assignment, err := h.assignmentService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "assignment")
	}
	// F-013: assignment must belong to the URL's :course_id. Without
	// this a teacher in course A could PUT
	// /courses/A/assignments/<id from course B same tenant> and edit
	// course B's assignment.
	if assignment.CourseID != uint(courseID) {
		return responses.NotFound(c, "assignment")
	}
	// A soft-deleted assignment is terminal — `deleted` has no outgoing
	// transition in the workflow state machine. FindByID does not exclude
	// deleted rows, so without this guard a PUT {"published":true} would
	// resurrect it to `published`, bypassing the state machine entirely.
	// Treat a deleted assignment as gone.
	if assignment.WorkflowState == "deleted" {
		return responses.NotFound(c, "assignment")
	}

	var input struct {
		Assignment struct {
			Name               *string    `json:"name"`
			Description        *string    `json:"description"`
			DueAt              *time.Time `json:"due_at"`
			UnlockAt           *time.Time `json:"unlock_at"`
			LockAt             *time.Time `json:"lock_at"`
			PointsPossible     *float64   `json:"points_possible"`
			GradingType        *string    `json:"grading_type"`
			SubmissionTypes    []string   `json:"submission_types"`
			Position           *int       `json:"position"`
			Published          *bool      `json:"published"`
			AssignmentGroupID  *uint      `json:"assignment_group_id"`
			AnonymousGrading   *bool      `json:"anonymous_grading"`
			PostPolicy         *string    `json:"post_policy"`
			PeerReviewsEnabled *bool      `json:"peer_reviews_enabled"`
			PeerReviewCount    *int       `json:"peer_review_count"`
			GroupCategoryID    *uint      `json:"group_category_id"`
			OnlyVisibleToOverrides *bool `json:"only_visible_to_overrides"`
			GradingStandardID      *uint `json:"grading_standard_id"`
		} `json:"assignment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	before := dueSnapshotOf(assignment)

	// B114: a pointer field can't tell "absent" from an explicit JSON
	// null, so clearing a date or turning off group assignment was
	// silently ignored. Detect explicit nulls from the raw body.
	var raw struct {
		Assignment map[string]json.RawMessage `json:"assignment"`
	}
	_ = json.Unmarshal(c.Body(), &raw)
	explicitNull := func(key string) bool {
		v, ok := raw.Assignment[key]
		return ok && strings.TrimSpace(string(v)) == "null"
	}
	if explicitNull("due_at") {
		assignment.DueAt = nil
	}
	if explicitNull("unlock_at") {
		assignment.UnlockAt = nil
	}
	if explicitNull("lock_at") {
		assignment.LockAt = nil
	}
	if explicitNull("points_possible") {
		assignment.PointsPossible = nil
	}
	if explicitNull("assignment_group_id") {
		assignment.AssignmentGroupID = nil
	}
	if explicitNull("group_category_id") {
		assignment.GroupCategoryID = nil
	}
	if explicitNull("grading_standard_id") {
		assignment.GradingStandardID = nil
	}

	if input.Assignment.Name != nil {
		assignment.Name = *input.Assignment.Name
	}
	if input.Assignment.Description != nil {
		assignment.Description = service.SanitizeHTML(*input.Assignment.Description)
	}
	if input.Assignment.DueAt != nil {
		assignment.DueAt = input.Assignment.DueAt
	}
	if input.Assignment.UnlockAt != nil {
		assignment.UnlockAt = input.Assignment.UnlockAt
	}
	if input.Assignment.LockAt != nil {
		assignment.LockAt = input.Assignment.LockAt
	}
	if input.Assignment.PointsPossible != nil {
		assignment.PointsPossible = input.Assignment.PointsPossible
	}
	if input.Assignment.GradingType != nil {
		assignment.GradingType = *input.Assignment.GradingType
	}
	if len(input.Assignment.SubmissionTypes) > 0 {
		assignment.SubmissionTypes = strings.Join(input.Assignment.SubmissionTypes, ",")
	}
	if input.Assignment.Position != nil {
		assignment.Position = *input.Assignment.Position
	}
	if input.Assignment.Published != nil {
		assignment.Published = *input.Assignment.Published
		if *input.Assignment.Published {
			assignment.WorkflowState = "published"
		} else {
			assignment.WorkflowState = "unpublished"
		}
	}
	if input.Assignment.AssignmentGroupID != nil {
		assignment.AssignmentGroupID = input.Assignment.AssignmentGroupID
	}
	if input.Assignment.AnonymousGrading != nil {
		assignment.AnonymousGrading = *input.Assignment.AnonymousGrading
	}
	if input.Assignment.PostPolicy != nil {
		assignment.PostPolicy = *input.Assignment.PostPolicy
	}
	if input.Assignment.PeerReviewsEnabled != nil {
		assignment.PeerReviewsEnabled = *input.Assignment.PeerReviewsEnabled
	}
	if input.Assignment.PeerReviewCount != nil {
		assignment.PeerReviewCount = *input.Assignment.PeerReviewCount
	}
	if input.Assignment.GroupCategoryID != nil {
		if *input.Assignment.GroupCategoryID == 0 {
			assignment.GroupCategoryID = nil
		} else {
			assignment.GroupCategoryID = input.Assignment.GroupCategoryID
		}
	}

	if input.Assignment.OnlyVisibleToOverrides != nil {
		assignment.OnlyVisibleToOverrides = *input.Assignment.OnlyVisibleToOverrides
	}
	if input.Assignment.GradingStandardID != nil {
		assignment.GradingStandardID = nonZeroUint(input.Assignment.GradingStandardID)
	}
	gradingTypeIn, postPolicyIn := "", ""
	if input.Assignment.GradingType != nil {
		gradingTypeIn = assignment.GradingType
	}
	if input.Assignment.PostPolicy != nil {
		postPolicyIn = assignment.PostPolicy
	}
	if msg := h.validateGradingFields(c, assignment.CourseID, gradingTypeIn, postPolicyIn, input.Assignment.GradingStandardID); msg != "" {
		return responses.BadRequest(c, msg)
	}
	if assignment.AnonymousGrading {
		// G018: anonymous assignments always post manually (Canvas).
		assignment.PostPolicy = "manual"
	}

	if err := h.assignmentService.Update(c.Context(), assignment); err != nil {
		return responses.InternalError(c, "Could not update assignment")
	}
	// G019 / G022: late and missing follow due-date and visibility edits.
	if h.gradingPolicy != nil && before != dueSnapshotOf(assignment) {
		_ = h.gradingPolicy.RefreshDueStatus(c.Context(), assignment.CourseID, []uint{assignment.ID})
	}

	return c.JSON(assignmentToJSON(assignment))
}

func (h *AssignmentHandler) DeleteAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	// SECURITY (F-011): load the assignment under the caller's tenant
	// AND verify it belongs to the URL's course. Pre-fix path called
	// h.assignmentService.Delete(id) with no scope, so a teacher in
	// course 10 could DELETE /courses/10/assignments/<id from any
	// other course/tenant>.
	assignment, err := h.assignmentService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "assignment")
	}
	if assignment.CourseID != uint(courseID) {
		return responses.NotFound(c, "assignment")
	}

	if err := h.assignmentService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete assignment")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// validateGradingFields checks the grading type, post policy and
// per-assignment grading scheme (empty / nil values are not checked).
// Returns a client error message, or "".
func (h *AssignmentHandler) validateGradingFields(c *fiber.Ctx, courseID uint, gradingType, postPolicy string, standardID *uint) string {
	if gradingType != "" && !service.ValidGradingType(gradingType) {
		return "grading_type must be points, percent, letter_grade, gpa_scale, pass_fail or not_graded"
	}
	if postPolicy != "" && postPolicy != "automatic" && postPolicy != "manual" {
		return "post_policy must be automatic or manual"
	}
	if standardID != nil && *standardID != 0 {
		if h.gradingPolicy == nil || !h.gradingPolicy.SchemeUsableByCourse(c.Context(), courseID, *standardID) {
			return "grading_standard_id is not a grading scheme of this course or its account"
		}
	}
	return ""
}

// nonZeroUint maps nil / 0 to nil.
func nonZeroUint(v *uint) *uint {
	if v == nil || *v == 0 {
		return nil
	}
	out := *v
	return &out
}

// dueSnapshot is what late / missing statuses depend on.
type dueSnapshot struct {
	due     string
	visible bool
	types   string
	state   models.AssignmentWorkflow
}

func dueSnapshotOf(a *models.Assignment) dueSnapshot {
	d := dueSnapshot{visible: a.OnlyVisibleToOverrides, types: a.SubmissionTypes, state: a.WorkflowState}
	if a.DueAt != nil {
		d.due = a.DueAt.UTC().Format(time.RFC3339Nano)
	}
	return d
}
