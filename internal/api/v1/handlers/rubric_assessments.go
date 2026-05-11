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

func (h *RubricAssessmentHandler) CreateAssessment(c *fiber.Ctx) error {
	associationID, err := c.ParamsInt("association_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid association ID")
	}

	assoc, err := h.rubricService.GetAssociation(c.Context(), uint(associationID))
	if err != nil {
		return responses.NotFound(c, "rubric association")
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
	assessmentID, err := c.ParamsInt("assessment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assessment ID")
	}

	assessment, err := h.rubricService.GetAssessment(c.Context(), uint(assessmentID))
	if err != nil {
		return responses.NotFound(c, "rubric assessment")
	}

	return c.JSON(rubricAssessmentToJSON(assessment))
}

func (h *RubricAssessmentHandler) UpdateAssessment(c *fiber.Ctx) error {
	assessmentID, err := c.ParamsInt("assessment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assessment ID")
	}

	assessment, err := h.rubricService.GetAssessment(c.Context(), uint(assessmentID))
	if err != nil {
		return responses.NotFound(c, "rubric assessment")
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
	associationID, err := c.ParamsInt("association_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid association ID")
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
