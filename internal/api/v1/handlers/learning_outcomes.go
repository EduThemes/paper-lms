package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type LearningOutcomeHandler struct {
	outcomeService *service.LearningOutcomeService
	alignmentRepo  repository.OutcomeAlignmentRepository
}

func NewLearningOutcomeHandler(outcomeService *service.LearningOutcomeService, alignmentRepo repository.OutcomeAlignmentRepository) *LearningOutcomeHandler {
	return &LearningOutcomeHandler{outcomeService: outcomeService, alignmentRepo: alignmentRepo}
}

func outcomeGroupToJSON(g *models.LearningOutcomeGroup) fiber.Map {
	return fiber.Map{
		"id":                      g.ID,
		"context_type":            g.ContextType,
		"context_id":              g.ContextID,
		"parent_outcome_group_id": g.ParentGroupID,
		"title":                   g.Title,
		"description":             g.Description,
		"workflow_state":          g.WorkflowState,
		"created_at":              g.CreatedAt,
		"updated_at":              g.UpdatedAt,
	}
}

func outcomeToJSON(o *models.LearningOutcome) fiber.Map {
	return fiber.Map{
		"id":                 o.ID,
		"context_type":       o.ContextType,
		"context_id":         o.ContextID,
		"outcome_group_id":   o.OutcomeGroupID,
		"title":              o.Title,
		"display_name":       o.DisplayName,
		"description":        o.Description,
		"calculation_method": o.CalculationMethod,
		"calculation_int":    o.CalculationInt,
		"mastery_points":     o.MasteryPoints,
		"points_possible":    o.PointsPossible,
		"ratings":            o.RatingsData,
		"workflow_state":     o.WorkflowState,
		"created_at":         o.CreatedAt,
		"updated_at":         o.UpdatedAt,
	}
}

func outcomeResultToJSON(r *models.LearningOutcomeResult) fiber.Map {
	return fiber.Map{
		"id":                    r.ID,
		"user_id":               r.UserID,
		"learning_outcome_id":   r.LearningOutcomeID,
		"context_type":          r.ContextType,
		"context_id":            r.ContextID,
		"associated_asset_type": r.AssociatedAssetType,
		"associated_asset_id":   r.AssociatedAssetID,
		"score":                 r.Score,
		"possible":              r.Possible,
		"mastery":               r.Mastery,
		"percent":               r.Percent,
		"attempt":               r.Attempt,
		"assessed_at":           r.AssessedAt,
		"submitted_at":          r.SubmittedAt,
		"title":                 r.Title,
		"created_at":            r.CreatedAt,
		"updated_at":            r.UpdatedAt,
	}
}

// Outcome Group endpoints

func (h *LearningOutcomeHandler) ListGroups(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.outcomeService.ListGroups(c.Context(), "Course", uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch outcome groups")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	groups := make([]fiber.Map, len(result.Items))
	for i, g := range result.Items {
		groups[i] = outcomeGroupToJSON(&g)
	}

	return c.JSON(groups)
}

func (h *LearningOutcomeHandler) CreateGroup(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Title         string `json:"title"`
		Description   string `json:"description"`
		ParentGroupID *uint  `json:"parent_outcome_group_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	group := &models.LearningOutcomeGroup{
		ContextType:   "Course",
		ContextID:     uint(courseID),
		Title:         input.Title,
		Description:   input.Description,
		ParentGroupID: input.ParentGroupID,
	}

	if err := h.outcomeService.CreateGroup(c.Context(), group); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(outcomeGroupToJSON(group))
}

func (h *LearningOutcomeHandler) GetGroup(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	group, err := h.outcomeService.GetGroup(c.Context(), uint(groupID))
	if err != nil {
		return responses.NotFound(c, "outcome group")
	}

	return c.JSON(outcomeGroupToJSON(group))
}

func (h *LearningOutcomeHandler) UpdateGroup(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	group, err := h.outcomeService.GetGroup(c.Context(), uint(groupID))
	if err != nil {
		return responses.NotFound(c, "outcome group")
	}

	var input struct {
		Title         *string `json:"title"`
		Description   *string `json:"description"`
		ParentGroupID *uint   `json:"parent_outcome_group_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Title != nil {
		group.Title = *input.Title
	}
	if input.Description != nil {
		group.Description = *input.Description
	}
	if input.ParentGroupID != nil {
		group.ParentGroupID = input.ParentGroupID
	}

	if err := h.outcomeService.UpdateGroup(c.Context(), group); err != nil {
		return responses.InternalError(c, "Could not update outcome group")
	}

	return c.JSON(outcomeGroupToJSON(group))
}

func (h *LearningOutcomeHandler) DeleteGroup(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	if err := h.outcomeService.DeleteGroup(c.Context(), uint(groupID)); err != nil {
		return responses.InternalError(c, "Could not delete outcome group")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// Outcome endpoints

func (h *LearningOutcomeHandler) ListOutcomes(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.outcomeService.ListOutcomes(c.Context(), uint(groupID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch outcomes")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	outcomes := make([]fiber.Map, len(result.Items))
	for i, o := range result.Items {
		outcomes[i] = outcomeToJSON(&o)
	}

	return c.JSON(outcomes)
}

func (h *LearningOutcomeHandler) CreateOutcome(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	var input struct {
		Title             string  `json:"title"`
		DisplayName       string  `json:"display_name"`
		Description       string  `json:"description"`
		CalculationMethod string  `json:"calculation_method"`
		CalculationInt    int     `json:"calculation_int"`
		MasteryPoints     float64 `json:"mastery_points"`
		PointsPossible    float64 `json:"points_possible"`
		Ratings           string  `json:"ratings"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	outcome := &models.LearningOutcome{
		ContextType:       "Course",
		ContextID:         uint(courseID),
		OutcomeGroupID:    uint(groupID),
		Title:             input.Title,
		DisplayName:       input.DisplayName,
		Description:       input.Description,
		CalculationMethod: input.CalculationMethod,
		CalculationInt:    input.CalculationInt,
		MasteryPoints:     input.MasteryPoints,
		PointsPossible:    input.PointsPossible,
		RatingsData:       input.Ratings,
	}

	if err := h.outcomeService.CreateOutcome(c.Context(), outcome); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(outcomeToJSON(outcome))
}

func (h *LearningOutcomeHandler) GetOutcome(c *fiber.Ctx) error {
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}

	outcome, err := h.outcomeService.GetOutcome(c.Context(), uint(outcomeID))
	if err != nil {
		return responses.NotFound(c, "outcome")
	}

	return c.JSON(outcomeToJSON(outcome))
}

func (h *LearningOutcomeHandler) UpdateOutcome(c *fiber.Ctx) error {
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}

	outcome, err := h.outcomeService.GetOutcome(c.Context(), uint(outcomeID))
	if err != nil {
		return responses.NotFound(c, "outcome")
	}

	var input struct {
		Title             *string  `json:"title"`
		DisplayName       *string  `json:"display_name"`
		Description       *string  `json:"description"`
		CalculationMethod *string  `json:"calculation_method"`
		CalculationInt    *int     `json:"calculation_int"`
		MasteryPoints     *float64 `json:"mastery_points"`
		PointsPossible    *float64 `json:"points_possible"`
		Ratings           *string  `json:"ratings"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Title != nil {
		outcome.Title = *input.Title
	}
	if input.DisplayName != nil {
		outcome.DisplayName = *input.DisplayName
	}
	if input.Description != nil {
		outcome.Description = *input.Description
	}
	if input.CalculationMethod != nil {
		outcome.CalculationMethod = *input.CalculationMethod
	}
	if input.CalculationInt != nil {
		outcome.CalculationInt = *input.CalculationInt
	}
	if input.MasteryPoints != nil {
		outcome.MasteryPoints = *input.MasteryPoints
	}
	if input.PointsPossible != nil {
		outcome.PointsPossible = *input.PointsPossible
	}
	if input.Ratings != nil {
		outcome.RatingsData = *input.Ratings
	}

	if err := h.outcomeService.UpdateOutcome(c.Context(), outcome); err != nil {
		return responses.InternalError(c, "Could not update outcome")
	}

	return c.JSON(outcomeToJSON(outcome))
}

func (h *LearningOutcomeHandler) DeleteOutcome(c *fiber.Ctx) error {
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}

	if err := h.outcomeService.DeleteOutcome(c.Context(), uint(outcomeID)); err != nil {
		return responses.InternalError(c, "Could not delete outcome")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// Result endpoints

func (h *LearningOutcomeHandler) ListResults(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID := c.QueryInt("user_id", 0)
	if userID > 0 {
		results, err := h.outcomeService.ListResultsByUserAndContext(c.Context(), uint(userID), "Course", uint(courseID))
		if err != nil {
			return responses.InternalError(c, "Could not fetch outcome results")
		}

		items := make([]fiber.Map, len(results))
		for i, r := range results {
			items[i] = outcomeResultToJSON(&r)
		}

		return c.JSON(fiber.Map{
			"outcome_results": items,
		})
	}

	// If no user_id, list all results for all outcomes in this course
	params := middleware.GetPagination(c)

	// Get all outcomes for the course first
	outcomes, err := h.outcomeService.ListOutcomesByContext(c.Context(), "Course", uint(courseID), repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return responses.InternalError(c, "Could not fetch outcomes")
	}

	var allResults []fiber.Map
	for _, outcome := range outcomes.Items {
		results, err := h.outcomeService.ListResultsByOutcome(c.Context(), outcome.ID, params)
		if err != nil {
			continue
		}
		for _, r := range results.Items {
			allResults = append(allResults, outcomeResultToJSON(&r))
		}
	}

	return c.JSON(fiber.Map{
		"outcome_results": allResults,
	})
}

func (h *LearningOutcomeHandler) CreateResult(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		UserID              uint     `json:"user_id"`
		LearningOutcomeID   uint     `json:"learning_outcome_id"`
		AssociatedAssetType string   `json:"associated_asset_type"`
		AssociatedAssetID   uint     `json:"associated_asset_id"`
		Score               *float64 `json:"score"`
		Possible            *float64 `json:"possible"`
		Attempt             int      `json:"attempt"`
		Title               string   `json:"title"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	result := &models.LearningOutcomeResult{
		UserID:              input.UserID,
		LearningOutcomeID:   input.LearningOutcomeID,
		ContextType:         "Course",
		ContextID:           uint(courseID),
		AssociatedAssetType: input.AssociatedAssetType,
		AssociatedAssetID:   input.AssociatedAssetID,
		Score:               input.Score,
		Possible:            input.Possible,
		Attempt:             input.Attempt,
		Title:               input.Title,
	}

	if err := h.outcomeService.CreateResult(c.Context(), result); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(outcomeResultToJSON(result))
}

// Mastery Gradebook endpoint

func (h *LearningOutcomeHandler) GetMasteryGradebook(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	rollups, outcomes, err := h.outcomeService.GetMasteryGradebook(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch mastery gradebook")
	}

	// Format outcomes for the response
	outcomeList := make([]fiber.Map, len(outcomes))
	for i, o := range outcomes {
		outcomeList[i] = outcomeToJSON(&o)
	}

	// Format rollups for the response
	rollupList := make([]fiber.Map, len(rollups))
	for i, r := range rollups {
		scores := make([]fiber.Map, len(r.Scores))
		for j, s := range r.Scores {
			scores[j] = fiber.Map{
				"outcome_id": s.OutcomeID,
				"score":      s.Score,
				"count":      s.Count,
				"mastery":    s.Mastery,
				"title":      s.Title,
			}
		}
		rollupList[i] = fiber.Map{
			"links": fiber.Map{
				"user": r.StudentID,
			},
			"scores": scores,
		}
	}

	return c.JSON(fiber.Map{
		"rollups":         rollupList,
		"linked": fiber.Map{
			"outcomes": outcomeList,
		},
	})
}

// Alignment endpoints

func (h *LearningOutcomeHandler) CreateAlignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		LearningOutcomeID uint `json:"learning_outcome_id"`
		AssignmentID      uint `json:"assignment_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.LearningOutcomeID == 0 || input.AssignmentID == 0 {
		return responses.BadRequest(c, "learning_outcome_id and assignment_id are required")
	}

	alignment := &models.OutcomeAlignment{
		LearningOutcomeID: input.LearningOutcomeID,
		AssignmentID:      input.AssignmentID,
		CourseID:          uint(courseID),
	}

	if err := h.alignmentRepo.Create(c.Context(), alignment); err != nil {
		return responses.InternalError(c, "Could not create alignment")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":                  alignment.ID,
		"learning_outcome_id": alignment.LearningOutcomeID,
		"assignment_id":       alignment.AssignmentID,
		"course_id":           alignment.CourseID,
	})
}

func (h *LearningOutcomeHandler) DeleteAlignment(c *fiber.Ctx) error {
	alignmentID, err := c.ParamsInt("alignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid alignment ID")
	}

	if err := h.alignmentRepo.Delete(c.Context(), uint(alignmentID)); err != nil {
		return responses.InternalError(c, "Could not delete alignment")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *LearningOutcomeHandler) ListAlignments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	assignmentID := c.QueryInt("assignment_id", 0)
	var alignments []models.OutcomeAlignment

	if assignmentID > 0 {
		alignments, err = h.alignmentRepo.ListByAssignmentID(c.Context(), uint(assignmentID))
	} else {
		alignments, err = h.alignmentRepo.ListByCourseID(c.Context(), uint(courseID))
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch alignments")
	}

	result := make([]fiber.Map, len(alignments))
	for i, a := range alignments {
		result[i] = fiber.Map{
			"id":                  a.ID,
			"learning_outcome_id": a.LearningOutcomeID,
			"assignment_id":       a.AssignmentID,
			"course_id":           a.CourseID,
		}
	}

	return c.JSON(result)
}
