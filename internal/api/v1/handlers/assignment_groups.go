package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AssignmentGroupHandler struct {
	groupService *service.AssignmentGroupService
}

func NewAssignmentGroupHandler(groupService *service.AssignmentGroupService) *AssignmentGroupHandler {
	return &AssignmentGroupHandler{groupService: groupService}
}

func assignmentGroupToJSON(g *models.AssignmentGroup) fiber.Map {
	return fiber.Map{
		"id":             g.ID,
		"course_id":      g.CourseID,
		"name":           g.Name,
		"position":       g.Position,
		"group_weight":   g.GroupWeight,
		"rules":          g.Rules,
		"workflow_state": g.WorkflowState,
		"created_at":     g.CreatedAt,
		"updated_at":     g.UpdatedAt,
	}
}

func (h *AssignmentGroupHandler) ListAssignmentGroups(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.groupService.ListByCourse(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignment groups")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	groups := make([]fiber.Map, len(result.Items))
	for i, g := range result.Items {
		groups[i] = assignmentGroupToJSON(&g)
	}

	return c.JSON(groups)
}

func (h *AssignmentGroupHandler) CreateAssignmentGroup(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		AssignmentGroup struct {
			Name        string  `json:"name"`
			GroupWeight float64 `json:"group_weight"`
			Position    int     `json:"position"`
			Rules       string  `json:"rules"`
		} `json:"assignment_group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	group := &models.AssignmentGroup{
		CourseID:    uint(courseID),
		Name:        input.AssignmentGroup.Name,
		GroupWeight: input.AssignmentGroup.GroupWeight,
		Position:    input.AssignmentGroup.Position,
		Rules:       input.AssignmentGroup.Rules,
	}

	if err := h.groupService.Create(c.Context(), group); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(assignmentGroupToJSON(group))
}

func (h *AssignmentGroupHandler) GetAssignmentGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment group ID")
	}

	group, err := h.groupService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "assignment group")
	}

	return c.JSON(assignmentGroupToJSON(group))
}

func (h *AssignmentGroupHandler) UpdateAssignmentGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment group ID")
	}

	group, err := h.groupService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "assignment group")
	}

	var input struct {
		AssignmentGroup struct {
			Name        *string  `json:"name"`
			GroupWeight *float64 `json:"group_weight"`
			Position    *int     `json:"position"`
			Rules       *string  `json:"rules"`
		} `json:"assignment_group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.AssignmentGroup.Name != nil {
		group.Name = *input.AssignmentGroup.Name
	}
	if input.AssignmentGroup.GroupWeight != nil {
		group.GroupWeight = *input.AssignmentGroup.GroupWeight
	}
	if input.AssignmentGroup.Position != nil {
		group.Position = *input.AssignmentGroup.Position
	}
	if input.AssignmentGroup.Rules != nil {
		group.Rules = *input.AssignmentGroup.Rules
	}

	if err := h.groupService.Update(c.Context(), group); err != nil {
		return responses.InternalError(c, "Could not update assignment group")
	}

	return c.JSON(assignmentGroupToJSON(group))
}

func (h *AssignmentGroupHandler) DeleteAssignmentGroup(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment group ID")
	}

	if err := h.groupService.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete assignment group")
	}

	return c.JSON(fiber.Map{"delete": true})
}
