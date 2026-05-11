package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type MasteryPathHandler struct {
	svc *service.MasteryPathService
}

func NewMasteryPathHandler(svc *service.MasteryPathService) *MasteryPathHandler {
	return &MasteryPathHandler{svc: svc}
}

// ---- JSON serializers --------------------------------------------------

func ruleToJSON(rule *models.ConditionalReleaseRule) fiber.Map {
	ranges := make([]fiber.Map, 0, len(rule.ScoringRanges))
	for i := range rule.ScoringRanges {
		ranges = append(ranges, scoringRangeToJSON(&rule.ScoringRanges[i]))
	}
	return fiber.Map{
		"id":                    rule.ID,
		"course_id":             rule.CourseID,
		"trigger_assignment_id": rule.TriggerAssignmentID,
		"workflow_state":        rule.WorkflowState,
		"created_at":            rule.CreatedAt,
		"updated_at":            rule.UpdatedAt,
		"scoring_ranges":        ranges,
	}
}

func scoringRangeToJSON(sr *models.ConditionalReleaseScoringRange) fiber.Map {
	sets := make([]fiber.Map, 0, len(sr.AssignmentSets))
	for i := range sr.AssignmentSets {
		sets = append(sets, assignmentSetToJSON(&sr.AssignmentSets[i]))
	}
	return fiber.Map{
		"id":              sr.ID,
		"rule_id":         sr.RuleID,
		"lower_bound":     sr.LowerBound,
		"upper_bound":     sr.UpperBound,
		"position":        sr.Position,
		"assignment_sets": sets,
	}
}

func assignmentSetToJSON(set *models.ConditionalReleaseAssignmentSet) fiber.Map {
	assocs := make([]fiber.Map, 0, len(set.Associations))
	for _, a := range set.Associations {
		assocs = append(assocs, fiber.Map{
			"id":            a.ID,
			"assignment_id": a.AssignmentID,
			"position":      a.Position,
		})
	}
	return fiber.Map{
		"id":                          set.ID,
		"scoring_range_id":            set.ScoringRangeID,
		"position":                    set.Position,
		"assignment_set_associations": assocs,
	}
}

// ---- Handlers ----------------------------------------------------------

// GET /courses/:course_id/mastery_paths/rules
func (h *MasteryPathHandler) ListRules(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	rules, err := h.svc.ListRules(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not load mastery paths rules")
	}
	out := make([]fiber.Map, len(rules))
	for i := range rules {
		out[i] = ruleToJSON(&rules[i])
	}
	return c.JSON(out)
}

// GET /courses/:course_id/mastery_paths/rules/:assignment_id
func (h *MasteryPathHandler) GetRuleForAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil || assignmentID <= 0 {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	rule, err := h.svc.GetRuleForAssignment(c.Context(), uint(courseID), uint(assignmentID))
	if err != nil {
		return responses.InternalError(c, "Could not load mastery path rule")
	}
	if rule == nil {
		return responses.NotFound(c, "mastery path rule")
	}
	return c.JSON(ruleToJSON(rule))
}

type createRuleRequest struct {
	TriggerAssignmentID uint                  `json:"trigger_assignment_id"`
	ScoringRanges       []service.RangeInput  `json:"scoring_ranges"`
}

// POST /courses/:course_id/mastery_paths/rules
func (h *MasteryPathHandler) CreateRule(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var req createRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return responses.BadRequest(c, "Invalid JSON body")
	}
	if req.TriggerAssignmentID == 0 {
		return responses.BadRequest(c, "trigger_assignment_id is required")
	}
	rule, err := h.svc.CreateRule(c.Context(), uint(courseID), req.TriggerAssignmentID, req.ScoringRanges)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(ruleToJSON(rule))
}

type replaceRuleRequest struct {
	ScoringRanges []service.RangeInput `json:"scoring_ranges"`
}

// PUT /courses/:course_id/mastery_paths/rules/:rule_id
func (h *MasteryPathHandler) ReplaceRule(c *fiber.Ctx) error {
	ruleID, err := c.ParamsInt("rule_id")
	if err != nil || ruleID <= 0 {
		return responses.BadRequest(c, "Invalid rule ID")
	}
	var req replaceRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return responses.BadRequest(c, "Invalid JSON body")
	}
	rule, err := h.svc.ReplaceRule(c.Context(), uint(ruleID), req.ScoringRanges)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(ruleToJSON(rule))
}

// DELETE /courses/:course_id/mastery_paths/rules/:rule_id
func (h *MasteryPathHandler) DeleteRule(c *fiber.Ctx) error {
	ruleID, err := c.ParamsInt("rule_id")
	if err != nil || ruleID <= 0 {
		return responses.BadRequest(c, "Invalid rule ID")
	}
	if err := h.svc.DeleteRule(c.Context(), uint(ruleID)); err != nil {
		return responses.InternalError(c, "Could not delete rule")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
