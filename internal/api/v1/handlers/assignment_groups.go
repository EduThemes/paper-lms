package handlers

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type AssignmentGroupHandler struct {
	groupService *service.AssignmentGroupService
}

func NewAssignmentGroupHandler(groupService *service.AssignmentGroupService) *AssignmentGroupHandler {
	return &AssignmentGroupHandler{groupService: groupService}
}

// assignmentGroupToJSON returns rules as an object ({drop_lowest,
// drop_highest, never_drop}), like Canvas.
func assignmentGroupToJSON(g *models.AssignmentGroup) fiber.Map {
	rules := service.ParseDropRules(g.Rules)
	if rules.NeverDrop == nil {
		rules.NeverDrop = []uint{}
	}
	return fiber.Map{
		"id":             g.ID,
		"course_id":      g.CourseID,
		"name":           g.Name,
		"position":       g.Position,
		"group_weight":   g.GroupWeight,
		"rules":          rules,
		"workflow_state": g.WorkflowState,
		"created_at":     g.CreatedAt,
		"updated_at":     g.UpdatedAt,
	}
}

// parseRulesInput accepts rules as an object or as a JSON / Canvas-format
// string and returns the stored form (G020). Negative counts are refused.
func parseRulesInput(raw json.RawMessage) (string, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return "{}", true
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if json.Unmarshal(raw, &str) != nil {
			return "", false
		}
		s = strings.TrimSpace(str)
		if s == "" {
			return "{}", true
		}
	}
	if strings.HasPrefix(s, "{") {
		var probe map[string]json.RawMessage
		if json.Unmarshal([]byte(s), &probe) != nil {
			return "", false
		}
		for _, k := range []string{"drop_lowest", "drop_highest"} {
			if v, ok := probe[k]; ok {
				var n float64
				if json.Unmarshal(v, &n) == nil && (n < 0 || n != math.Trunc(n)) {
					return "", false
				}
			}
		}
	}
	rules := service.ParseDropRules(s)
	return service.MarshalDropRules(rules), true
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
			Name        string          `json:"name"`
			GroupWeight float64         `json:"group_weight"`
			Position    int             `json:"position"`
			Rules       json.RawMessage `json:"rules"`
		} `json:"assignment_group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.AssignmentGroup.GroupWeight < 0 || math.IsNaN(input.AssignmentGroup.GroupWeight) {
		return responses.BadRequest(c, "group_weight cannot be negative")
	}
	rules, ok := parseRulesInput(input.AssignmentGroup.Rules)
	if !ok {
		return responses.BadRequest(c, "rules must have whole, non-negative drop_lowest / drop_highest counts")
	}

	group := &models.AssignmentGroup{
		CourseID:    uint(courseID),
		Name:        strings.TrimSpace(input.AssignmentGroup.Name),
		GroupWeight: input.AssignmentGroup.GroupWeight,
		Position:    input.AssignmentGroup.Position,
		Rules:       rules,
	}

	if err := h.groupService.Create(c.Context(), group); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(assignmentGroupToJSON(group))
}

// loadGroupInCourse loads :id under the caller's tenant and ties it to
// :course_id (F-013). On a miss it writes a 404 and returns wrote=true.
func (h *AssignmentGroupHandler) loadGroupInCourse(c *fiber.Ctx) (*models.AssignmentGroup, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid assignment group ID")
		return nil, true
	}
	group, err := h.groupService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil || group == nil || group.CourseID != uint(courseID) || group.WorkflowState == "deleted" {
		_ = responses.NotFound(c, "assignment group")
		return nil, true
	}
	return group, false
}

func (h *AssignmentGroupHandler) GetAssignmentGroup(c *fiber.Ctx) error {
	group, wrote := h.loadGroupInCourse(c)
	if wrote {
		return nil
	}
	return c.JSON(assignmentGroupToJSON(group))
}

func (h *AssignmentGroupHandler) UpdateAssignmentGroup(c *fiber.Ctx) error {
	group, wrote := h.loadGroupInCourse(c)
	if wrote {
		return nil
	}

	var input struct {
		AssignmentGroup struct {
			Name        *string         `json:"name"`
			GroupWeight *float64        `json:"group_weight"`
			Position    *int            `json:"position"`
			Rules       json.RawMessage `json:"rules"`
		} `json:"assignment_group"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.AssignmentGroup.Name != nil {
		name := strings.TrimSpace(*input.AssignmentGroup.Name)
		if name == "" {
			return responses.BadRequest(c, "assignment group name is required")
		}
		group.Name = name
	}
	if input.AssignmentGroup.GroupWeight != nil {
		if *input.AssignmentGroup.GroupWeight < 0 || math.IsNaN(*input.AssignmentGroup.GroupWeight) {
			return responses.BadRequest(c, "group_weight cannot be negative")
		}
		group.GroupWeight = *input.AssignmentGroup.GroupWeight
	}
	if input.AssignmentGroup.Position != nil {
		group.Position = *input.AssignmentGroup.Position
	}
	if len(input.AssignmentGroup.Rules) > 0 {
		rules, ok := parseRulesInput(input.AssignmentGroup.Rules)
		if !ok {
			return responses.BadRequest(c, "rules must have whole, non-negative drop_lowest / drop_highest counts")
		}
		group.Rules = rules
	}
	if strings.TrimSpace(group.Rules) == "" {
		group.Rules = "{}"
	}

	if err := h.groupService.Update(c.Context(), group); err != nil {
		return responses.InternalError(c, "Could not update assignment group")
	}

	return c.JSON(assignmentGroupToJSON(group))
}

func (h *AssignmentGroupHandler) DeleteAssignmentGroup(c *fiber.Ctx) error {
	// Load under the caller's tenant and tie to :course_id before the
	// destructive step (the old path deleted any id in any tenant).
	group, wrote := h.loadGroupInCourse(c)
	if wrote {
		return nil
	}

	if err := h.groupService.Delete(c.Context(), group.ID); err != nil {
		return responses.InternalError(c, "Could not delete assignment group")
	}

	return c.JSON(fiber.Map{"delete": true})
}
