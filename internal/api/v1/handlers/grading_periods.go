package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type GradingPeriodHandler struct {
	gradingPeriodService *service.GradingPeriodService
}

func NewGradingPeriodHandler(gradingPeriodService *service.GradingPeriodService) *GradingPeriodHandler {
	return &GradingPeriodHandler{gradingPeriodService: gradingPeriodService}
}

func gradingPeriodGroupToJSON(g *models.GradingPeriodGroup) fiber.Map {
	return fiber.Map{
		"id":         g.ID,
		"account_id": g.AccountID,
		"title":      g.Title,
		"weighted":   g.Weighted,
		"display_totals_for_all_grading_periods": g.DisplayTotals,
		"workflow_state": g.WorkflowState,
		"created_at":     g.CreatedAt,
		"updated_at":     g.UpdatedAt,
	}
}

func gradingPeriodToJSON(p *models.GradingPeriod) fiber.Map {
	return fiber.Map{
		"id":                      p.ID,
		"grading_period_group_id": p.GradingPeriodGroupID,
		"title":                   p.Title,
		"start_date":              p.StartDate,
		"end_date":                p.EndDate,
		"close_date":              p.CloseDate,
		"weight":                  p.Weight,
		"is_closed":               p.IsClosed,
		"workflow_state":          p.WorkflowState,
		"created_at":              p.CreatedAt,
		"updated_at":              p.UpdatedAt,
	}
}

// Group handlers

func (h *GradingPeriodHandler) ListGroups(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.gradingPeriodService.ListGroupsByAccount(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch grading period groups")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	groups := make([]fiber.Map, len(result.Items))
	for i, g := range result.Items {
		groups[i] = gradingPeriodGroupToJSON(&g)
	}

	return c.JSON(fiber.Map{"grading_period_groups": groups})
}

func (h *GradingPeriodHandler) CreateGroup(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	var input struct {
		GradingPeriodGroup struct {
			Title         string `json:"title"`
			Weighted      *bool  `json:"weighted"`
			DisplayTotals *bool  `json:"display_totals_for_all_grading_periods"`
		} `json:"grading_period_group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	group := &models.GradingPeriodGroup{
		AccountID: uint(accountID),
		Title:     input.GradingPeriodGroup.Title,
	}

	if input.GradingPeriodGroup.Weighted != nil {
		group.Weighted = *input.GradingPeriodGroup.Weighted
	}
	if input.GradingPeriodGroup.DisplayTotals != nil {
		group.DisplayTotals = *input.GradingPeriodGroup.DisplayTotals
	}

	if err := h.gradingPeriodService.CreateGroup(c.Context(), group); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(gradingPeriodGroupToJSON(group))
}

func (h *GradingPeriodHandler) GetGroup(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	group, err := h.gradingPeriodService.GetGroup(c.Context(), uint(groupID))
	if err != nil {
		return responses.NotFound(c, "grading period group")
	}

	return c.JSON(gradingPeriodGroupToJSON(group))
}

func (h *GradingPeriodHandler) UpdateGroup(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	group, err := h.gradingPeriodService.GetGroup(c.Context(), uint(groupID))
	if err != nil {
		return responses.NotFound(c, "grading period group")
	}

	var input struct {
		GradingPeriodGroup struct {
			Title         *string `json:"title"`
			Weighted      *bool   `json:"weighted"`
			DisplayTotals *bool   `json:"display_totals_for_all_grading_periods"`
		} `json:"grading_period_group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.GradingPeriodGroup.Title != nil {
		group.Title = *input.GradingPeriodGroup.Title
	}
	if input.GradingPeriodGroup.Weighted != nil {
		group.Weighted = *input.GradingPeriodGroup.Weighted
	}
	if input.GradingPeriodGroup.DisplayTotals != nil {
		group.DisplayTotals = *input.GradingPeriodGroup.DisplayTotals
	}

	if err := h.gradingPeriodService.UpdateGroup(c.Context(), group); err != nil {
		return responses.InternalError(c, "Could not update grading period group")
	}

	return c.JSON(gradingPeriodGroupToJSON(group))
}

func (h *GradingPeriodHandler) DeleteGroup(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	if err := h.gradingPeriodService.DeleteGroup(c.Context(), uint(groupID)); err != nil {
		return responses.InternalError(c, "Could not delete grading period group")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// Period handlers

func (h *GradingPeriodHandler) ListPeriods(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	periods, err := h.gradingPeriodService.ListPeriodsByGroup(c.Context(), uint(groupID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch grading periods")
	}

	result := make([]fiber.Map, len(periods))
	for i, p := range periods {
		result[i] = gradingPeriodToJSON(&p)
	}

	return c.JSON(fiber.Map{"grading_periods": result})
}

func (h *GradingPeriodHandler) CreatePeriod(c *fiber.Ctx) error {
	groupID, err := c.ParamsInt("group_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid group ID")
	}

	var input struct {
		GradingPeriod struct {
			Title     string     `json:"title"`
			StartDate time.Time  `json:"start_date"`
			EndDate   time.Time  `json:"end_date"`
			CloseDate *time.Time `json:"close_date"`
			Weight    *float64   `json:"weight"`
		} `json:"grading_period"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	period := &models.GradingPeriod{
		GradingPeriodGroupID: uint(groupID),
		Title:                input.GradingPeriod.Title,
		StartDate:            input.GradingPeriod.StartDate,
		EndDate:              input.GradingPeriod.EndDate,
		CloseDate:            input.GradingPeriod.CloseDate,
		Weight:               input.GradingPeriod.Weight,
	}

	if err := h.gradingPeriodService.CreatePeriod(c.Context(), period); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(gradingPeriodToJSON(period))
}

func (h *GradingPeriodHandler) GetPeriod(c *fiber.Ctx) error {
	periodID, err := c.ParamsInt("period_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid period ID")
	}

	period, err := h.gradingPeriodService.GetPeriod(c.Context(), uint(periodID))
	if err != nil {
		return responses.NotFound(c, "grading period")
	}

	return c.JSON(gradingPeriodToJSON(period))
}

func (h *GradingPeriodHandler) UpdatePeriod(c *fiber.Ctx) error {
	periodID, err := c.ParamsInt("period_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid period ID")
	}

	period, err := h.gradingPeriodService.GetPeriod(c.Context(), uint(periodID))
	if err != nil {
		return responses.NotFound(c, "grading period")
	}

	var input struct {
		GradingPeriod struct {
			Title     *string    `json:"title"`
			StartDate *time.Time `json:"start_date"`
			EndDate   *time.Time `json:"end_date"`
			CloseDate *time.Time `json:"close_date"`
			Weight    *float64   `json:"weight"`
			IsClosed  *bool      `json:"is_closed"`
		} `json:"grading_period"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.GradingPeriod.Title != nil {
		period.Title = *input.GradingPeriod.Title
	}
	if input.GradingPeriod.StartDate != nil {
		period.StartDate = *input.GradingPeriod.StartDate
	}
	if input.GradingPeriod.EndDate != nil {
		period.EndDate = *input.GradingPeriod.EndDate
	}
	if input.GradingPeriod.CloseDate != nil {
		period.CloseDate = input.GradingPeriod.CloseDate
	}
	if input.GradingPeriod.Weight != nil {
		period.Weight = input.GradingPeriod.Weight
	}
	if input.GradingPeriod.IsClosed != nil {
		period.IsClosed = *input.GradingPeriod.IsClosed
	}

	if err := h.gradingPeriodService.UpdatePeriod(c.Context(), period); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(gradingPeriodToJSON(period))
}

func (h *GradingPeriodHandler) DeletePeriod(c *fiber.Ctx) error {
	periodID, err := c.ParamsInt("period_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid period ID")
	}

	if err := h.gradingPeriodService.DeletePeriod(c.Context(), uint(periodID)); err != nil {
		return responses.InternalError(c, "Could not delete grading period")
	}

	return c.JSON(fiber.Map{"delete": true})
}
