package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type RubricHandler struct {
	rubricService *service.RubricService
}

func NewRubricHandler(rubricService *service.RubricService) *RubricHandler {
	return &RubricHandler{rubricService: rubricService}
}

func rubricToJSON(r *models.Rubric) fiber.Map {
	return fiber.Map{
		"id":                          r.ID,
		"context_type":                r.ContextType,
		"context_id":                  r.ContextID,
		"title":                       r.Title,
		"description":                 r.Description,
		"data":                        r.Data,
		"points_possible":             r.PointsPossible,
		"free_form_criterion_comments": r.FreeFormCriterionComments,
		"hide_score_total":            r.HideScoreTotal,
		"hide_points":                 r.HidePoints,
		"workflow_state":              r.WorkflowState,
		"created_at":                  r.CreatedAt,
		"updated_at":                  r.UpdatedAt,
	}
}

func rubricAssociationToJSON(a *models.RubricAssociation) fiber.Map {
	return fiber.Map{
		"id":                   a.ID,
		"rubric_id":            a.RubricID,
		"association_id":       a.AssociationID,
		"association_type":     a.AssociationType,
		"context_type":         a.ContextType,
		"context_id":           a.ContextID,
		"purpose":              a.Purpose,
		"use_for_grading":      a.UseForGrading,
		"hide_score_total":     a.HideScoreTotal,
		"hide_points":          a.HidePoints,
		"hide_outcome_results": a.HideOutcomeResults,
		"created_at":           a.CreatedAt,
		"updated_at":           a.UpdatedAt,
	}
}

func (h *RubricHandler) ListCourseRubrics(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.rubricService.ListRubricsByContext(c.Context(), "Course", uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch rubrics")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	rubrics := make([]fiber.Map, len(result.Items))
	for i, r := range result.Items {
		rubrics[i] = rubricToJSON(&r)
	}

	return c.JSON(rubrics)
}

func (h *RubricHandler) CreateCourseRubric(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Rubric struct {
			Title                     string  `json:"title"`
			Description               string  `json:"description"`
			Data                      string  `json:"data"`
			PointsPossible            float64 `json:"points_possible"`
			FreeFormCriterionComments bool    `json:"free_form_criterion_comments"`
			HideScoreTotal            bool    `json:"hide_score_total"`
			HidePoints                bool    `json:"hide_points"`
		} `json:"rubric"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	rubric := &models.Rubric{
		ContextType:               "Course",
		ContextID:                 uint(courseID),
		Title:                     input.Rubric.Title,
		Description:               input.Rubric.Description,
		Data:                      input.Rubric.Data,
		PointsPossible:            input.Rubric.PointsPossible,
		FreeFormCriterionComments: input.Rubric.FreeFormCriterionComments,
		HideScoreTotal:            input.Rubric.HideScoreTotal,
		HidePoints:                input.Rubric.HidePoints,
	}

	if err := h.rubricService.CreateRubric(c.Context(), rubric); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(rubricToJSON(rubric))
}

func (h *RubricHandler) GetRubric(c *fiber.Ctx) error {
	rubricID, err := c.ParamsInt("rubric_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid rubric ID")
	}

	rubric, err := h.rubricService.GetRubric(c.Context(), uint(rubricID))
	if err != nil {
		return responses.NotFound(c, "rubric")
	}

	return c.JSON(rubricToJSON(rubric))
}

func (h *RubricHandler) UpdateRubric(c *fiber.Ctx) error {
	rubricID, err := c.ParamsInt("rubric_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid rubric ID")
	}

	rubric, err := h.rubricService.GetRubric(c.Context(), uint(rubricID))
	if err != nil {
		return responses.NotFound(c, "rubric")
	}

	var input struct {
		Rubric struct {
			Title                     *string  `json:"title"`
			Description               *string  `json:"description"`
			Data                      *string  `json:"data"`
			PointsPossible            *float64 `json:"points_possible"`
			FreeFormCriterionComments *bool    `json:"free_form_criterion_comments"`
			HideScoreTotal            *bool    `json:"hide_score_total"`
			HidePoints                *bool    `json:"hide_points"`
		} `json:"rubric"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Rubric.Title != nil {
		rubric.Title = *input.Rubric.Title
	}
	if input.Rubric.Description != nil {
		rubric.Description = *input.Rubric.Description
	}
	if input.Rubric.Data != nil {
		rubric.Data = *input.Rubric.Data
	}
	if input.Rubric.PointsPossible != nil {
		rubric.PointsPossible = *input.Rubric.PointsPossible
	}
	if input.Rubric.FreeFormCriterionComments != nil {
		rubric.FreeFormCriterionComments = *input.Rubric.FreeFormCriterionComments
	}
	if input.Rubric.HideScoreTotal != nil {
		rubric.HideScoreTotal = *input.Rubric.HideScoreTotal
	}
	if input.Rubric.HidePoints != nil {
		rubric.HidePoints = *input.Rubric.HidePoints
	}

	if err := h.rubricService.UpdateRubric(c.Context(), rubric); err != nil {
		return responses.InternalError(c, "Could not update rubric")
	}

	return c.JSON(rubricToJSON(rubric))
}

func (h *RubricHandler) DeleteRubric(c *fiber.Ctx) error {
	rubricID, err := c.ParamsInt("rubric_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid rubric ID")
	}

	if err := h.rubricService.DeleteRubric(c.Context(), uint(rubricID)); err != nil {
		return responses.InternalError(c, "Could not delete rubric")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *RubricHandler) GetAssignmentRubric(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	rubric, assoc, err := h.rubricService.GetRubricForAssignment(c.Context(), uint(assignmentID))
	if err != nil {
		return responses.NotFound(c, "rubric")
	}

	return c.JSON(fiber.Map{
		"rubric":              rubricToJSON(rubric),
		"rubric_association":  rubricAssociationToJSON(assoc),
	})
}

func (h *RubricHandler) AssociateRubric(c *fiber.Ctx) error {
	rubricID, err := c.ParamsInt("rubric_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid rubric ID")
	}

	var input struct {
		RubricAssociation struct {
			AssociationID   uint   `json:"association_id"`
			AssociationType string `json:"association_type"`
			UseForGrading   bool   `json:"use_for_grading"`
		} `json:"rubric_association"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.RubricAssociation.AssociationID == 0 {
		return responses.BadRequest(c, "association_id is required")
	}
	if input.RubricAssociation.AssociationType == "" {
		return responses.BadRequest(c, "association_type is required")
	}

	assoc, err := h.rubricService.CreateAssociation(
		c.Context(),
		uint(rubricID),
		input.RubricAssociation.AssociationID,
		input.RubricAssociation.AssociationType,
		input.RubricAssociation.UseForGrading,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(rubricAssociationToJSON(assoc))
}
