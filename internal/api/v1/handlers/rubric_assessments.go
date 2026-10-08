package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type RubricAssessmentHandler struct {
	rubricService *service.RubricService

	// Optional visibility collaborators (nil-safe): observer links and
	// the post state of the assessed submission.
	submissionService *service.SubmissionService
	observerService   *service.ObserverService
}

// SetVisibilityDeps wires the services used to decide which assessments a
// non-staff caller may read.
func (h *RubricAssessmentHandler) SetVisibilityDeps(submissionService *service.SubmissionService, observerService *service.ObserverService) {
	h.submissionService = submissionService
	h.observerService = observerService
}

// callerCanSeeAssessment applies the student-side visibility rules
// (B017). Course staff see every assessment. Anyone else sees only an
// assessment of themselves (or of a student they observe) or one they
// authored (a peer review); a grading assessment of an assignment stays
// hidden until that submission's grade is posted.
func (h *RubricAssessmentHandler) callerCanSeeAssessment(c *fiber.Ctx, assoc *models.RubricAssociation, a *models.RubricAssessment) bool {
	if callerIsCourseStaff(c) {
		return true
	}
	callerID, _ := c.Locals("user_id").(uint)
	if callerID == 0 {
		return false
	}
	if a.AssessorID == callerID && a.UserID != callerID {
		return true // the caller's own peer review of someone else
	}
	subject := a.UserID == callerID
	if !subject && h.observerService != nil {
		subject, _ = h.observerService.IsObserverOf(c.Context(), callerID, a.UserID)
	}
	if !subject {
		return false
	}
	if a.AssessmentType == "peer_review" || assoc == nil || assoc.AssociationType != "Assignment" {
		return true
	}
	if h.submissionService == nil {
		return true
	}
	sub, err := h.submissionService.GetByAssignmentAndUser(c.Context(), assoc.AssociationID, a.UserID, callerAccountID(c))
	return err == nil && sub != nil && sub.PostedAt != nil
}

func NewRubricAssessmentHandler(rubricService *service.RubricService) *RubricAssessmentHandler {
	return &RubricAssessmentHandler{rubricService: rubricService}
}

func rubricAssessmentToJSON(a *models.RubricAssessment) fiber.Map {
	return fiber.Map{
		"id":                    a.ID,
		"rubric_id":             a.RubricID,
		"rubric_association_id": a.RubricAssociationID,
		"user_id":               a.UserID,
		"assessor_id":           a.AssessorID,
		"score":                 a.Score,
		"data":                  a.Data,
		"assessment_type":       a.AssessmentType,
		"workflow_state":        a.WorkflowState,
		"created_at":            a.CreatedAt,
		"updated_at":            a.UpdatedAt,
	}
}

// requireAssociationInCourse loads :association_id and verifies it is a
// Course-context association whose course is :course_id (and therefore
// the caller's tenant — the route middleware pins :course_id to an
// enrollment). Returns wrote=true (404 written) on any miss so the caller
// short-circuits with `return nil`. Closes the nested-route IDOR.
func (h *RubricAssessmentHandler) requireAssociationInCourse(c *fiber.Ctx, associationID, courseID uint) (*models.RubricAssociation, bool) {
	assoc, err := h.rubricService.GetAssociation(c.Context(), associationID)
	if err != nil || assoc == nil || assoc.ContextType != "Course" || assoc.ContextID != courseID {
		_ = responses.NotFound(c, "rubric association")
		return nil, true
	}
	return assoc, false
}

// scopedAssessment loads :assessment_id and verifies it belongs to
// :association_id, which must itself be a Course-context association in
// :course_id and the caller's tenant. Returns wrote=true on any miss.
func (h *RubricAssessmentHandler) scopedAssessment(c *fiber.Ctx) (*models.RubricAssessment, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	associationID, err := c.ParamsInt("association_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid association ID")
		return nil, true
	}
	assessmentID, err := c.ParamsInt("assessment_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid assessment ID")
		return nil, true
	}
	assoc, wrote := h.requireAssociationInCourse(c, uint(associationID), uint(courseID))
	if wrote {
		return nil, true
	}
	assessment, err := h.rubricService.GetAssessment(c.Context(), uint(assessmentID))
	if err != nil || assessment.RubricAssociationID != uint(associationID) ||
		!h.callerCanSeeAssessment(c, assoc, assessment) {
		_ = responses.NotFound(c, "rubric assessment")
		return nil, true
	}
	return assessment, false
}

func (h *RubricAssessmentHandler) CreateAssessment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	associationID, err := c.ParamsInt("association_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid association ID")
	}

	assoc, wrote := h.requireAssociationInCourse(c, uint(associationID), uint(courseID))
	if wrote {
		return nil
	}

	var input struct {
		RubricAssessment struct {
			UserID         uint   `json:"user_id"`
			Data           string `json:"data"`
			AssessmentType string `json:"assessment_type"`
		} `json:"rubric_assessment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	assessorID, _ := c.Locals("user_id").(uint)

	assessment := &models.RubricAssessment{
		RubricID:            assoc.RubricID,
		RubricAssociationID: uint(associationID),
		UserID:              input.RubricAssessment.UserID,
		AssessorID:          assessorID,
		Data:                input.RubricAssessment.Data,
		AssessmentType:      input.RubricAssessment.AssessmentType,
	}

	if err := h.rubricService.CreateAssessment(c.Context(), assessment); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(rubricAssessmentToJSON(assessment))
}

func (h *RubricAssessmentHandler) GetAssessment(c *fiber.Ctx) error {
	assessment, wrote := h.scopedAssessment(c)
	if wrote {
		return nil
	}

	return c.JSON(rubricAssessmentToJSON(assessment))
}

func (h *RubricAssessmentHandler) UpdateAssessment(c *fiber.Ctx) error {
	assessment, wrote := h.scopedAssessment(c)
	if wrote {
		return nil
	}

	var input struct {
		RubricAssessment struct {
			Data           *string `json:"data"`
			AssessmentType *string `json:"assessment_type"`
		} `json:"rubric_assessment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.RubricAssessment.Data != nil {
		assessment.Data = *input.RubricAssessment.Data
	}
	if input.RubricAssessment.AssessmentType != nil {
		assessment.AssessmentType = *input.RubricAssessment.AssessmentType
	}

	if err := h.rubricService.UpdateAssessment(c.Context(), assessment); err != nil {
		return responses.InternalError(c, "Could not update rubric assessment")
	}

	return c.JSON(rubricAssessmentToJSON(assessment))
}

func (h *RubricAssessmentHandler) ListAssessments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	associationID, err := c.ParamsInt("association_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid association ID")
	}
	assoc, wrote := h.requireAssociationInCourse(c, uint(associationID), uint(courseID))
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	if !callerIsCourseStaff(c) {
		// B017: a student/observer sees only the assessments they may
		// read. Filter the whole association first, then paginate, so the
		// count never reveals classmates' assessments.
		all, err := h.rubricService.ListAssessmentsByAssociation(c.Context(), uint(associationID), repository.PaginationParams{Page: 1, PerPage: 10000})
		if err != nil {
			return responses.InternalError(c, "Could not fetch rubric assessments")
		}
		visible := make([]fiber.Map, 0)
		for i := range all.Items {
			if h.callerCanSeeAssessment(c, assoc, &all.Items[i]) {
				visible = append(visible, rubricAssessmentToJSON(&all.Items[i]))
			}
		}
		if params.Page < 1 {
			params.Page = 1
		}
		start := (params.Page - 1) * params.PerPage
		if start > len(visible) {
			start = len(visible)
		}
		end := start + params.PerPage
		if end > len(visible) {
			end = len(visible)
		}
		responses.SetPaginationHeaders(c, int64(len(visible)), params.Page, params.PerPage)
		return c.JSON(visible[start:end])
	}

	result, err := h.rubricService.ListAssessmentsByAssociation(c.Context(), uint(associationID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch rubric assessments")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	assessments := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		assessments[i] = rubricAssessmentToJSON(&a)
	}

	return c.JSON(assessments)
}
