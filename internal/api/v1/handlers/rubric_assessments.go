package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type RubricAssessmentHandler struct {
	rubricService *service.RubricService
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
	if _, wrote := h.requireAssociationInCourse(c, uint(associationID), uint(courseID)); wrote {
		return nil, true
	}
	assessment, err := h.rubricService.GetAssessment(c.Context(), uint(assessmentID))
	if err != nil || assessment.RubricAssociationID != uint(associationID) {
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
	if _, wrote := h.requireAssociationInCourse(c, uint(associationID), uint(courseID)); wrote {
		return nil
	}

	params := middleware.GetPagination(c)

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
