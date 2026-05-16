package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// OutcomeProficiencyHandler exposes Canvas-compatible Outcome Proficiency and
// Learning Mastery Gradebook endpoints.
type OutcomeProficiencyHandler struct {
	proficiency *service.OutcomeProficiencyService
	mastery     *service.MasteryGradebookService
}

func NewOutcomeProficiencyHandler(proficiency *service.OutcomeProficiencyService, mastery *service.MasteryGradebookService) *OutcomeProficiencyHandler {
	return &OutcomeProficiencyHandler{proficiency: proficiency, mastery: mastery}
}

func proficiencyRatingToJSON(r *models.OutcomeProficiencyRating) fiber.Map {
	return fiber.Map{
		"id":          r.ID,
		"description": r.Description,
		"points":      r.Points,
		"mastery":     r.Mastery,
		"color":       r.Color,
		"position":    r.Position,
	}
}

func proficiencyToJSON(p *models.OutcomeProficiency) fiber.Map {
	ratings := make([]fiber.Map, len(p.Ratings))
	for i := range p.Ratings {
		ratings[i] = proficiencyRatingToJSON(&p.Ratings[i])
	}
	return fiber.Map{
		"id":             p.ID,
		"context_type":   p.ContextType,
		"context_id":     p.ContextID,
		"workflow_state": p.WorkflowState,
		"ratings":        ratings,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}
}

type proficiencyInput struct {
	Ratings []struct {
		Description string  `json:"description"`
		Points      float64 `json:"points"`
		Mastery     bool    `json:"mastery"`
		Color       string  `json:"color"`
		Position    int     `json:"position"`
	} `json:"ratings"`
}

func (in *proficiencyInput) toRatings() []models.OutcomeProficiencyRating {
	ratings := make([]models.OutcomeProficiencyRating, len(in.Ratings))
	for i, r := range in.Ratings {
		color := r.Color
		if color == "" {
			color = "#999999"
		}
		ratings[i] = models.OutcomeProficiencyRating{
			Description: r.Description,
			Points:      r.Points,
			Mastery:     r.Mastery,
			Color:       color,
			Position:    r.Position,
		}
	}
	return ratings
}

// --- Account-level handlers ---

func (h *OutcomeProficiencyHandler) GetForAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	p, err := h.proficiency.Get(c.Context(), "Account", uint(id))
	if err != nil {
		return responses.InternalError(c, "Could not fetch proficiency")
	}
	return c.JSON(proficiencyToJSON(p))
}

func (h *OutcomeProficiencyHandler) SetForAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	var input proficiencyInput
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	p, err := h.proficiency.Set(c.Context(), "Account", uint(id), input.toRatings())
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(proficiencyToJSON(p))
}

func (h *OutcomeProficiencyHandler) DeleteForAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if err := h.proficiency.Reset(c.Context(), "Account", uint(id)); err != nil {
		return responses.InternalError(c, "Could not reset proficiency")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Course-level handlers ---

func (h *OutcomeProficiencyHandler) GetForCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	p, err := h.proficiency.Get(c.Context(), "Course", uint(id))
	if err != nil {
		return responses.InternalError(c, "Could not fetch proficiency")
	}
	return c.JSON(proficiencyToJSON(p))
}

func (h *OutcomeProficiencyHandler) SetForCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input proficiencyInput
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	p, err := h.proficiency.Set(c.Context(), "Course", uint(id), input.toRatings())
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(proficiencyToJSON(p))
}

func (h *OutcomeProficiencyHandler) DeleteForCourse(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	if err := h.proficiency.Reset(c.Context(), "Course", uint(id)); err != nil {
		return responses.InternalError(c, "Could not reset proficiency")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Learning Mastery Gradebook ---

func (h *OutcomeProficiencyHandler) LearningMasteryGradebook(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	gb, err := h.mastery.GetMasteryGradebook(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not build mastery gradebook")
	}
	students := make([]fiber.Map, len(gb.Students))
	for i, s := range gb.Students {
		students[i] = fiber.Map{"id": s.ID, "name": s.Name, "email": s.Email}
	}
	outcomes := make([]fiber.Map, len(gb.Outcomes))
	for i, o := range gb.Outcomes {
		outcomes[i] = fiber.Map{
			"id":              o.ID,
			"title":           o.Title,
			"display_name":    o.DisplayName,
			"mastery_points":  o.MasteryPoints,
			"points_possible": o.PointsPossible,
		}
	}
	cells := make([]fiber.Map, len(gb.Cells))
	for i, cell := range gb.Cells {
		m := fiber.Map{"user_id": cell.UserID, "outcome_id": cell.OutcomeID}
		if cell.Score != nil {
			m["score"] = *cell.Score
		}
		if cell.Possible != nil {
			m["possible"] = *cell.Possible
		}
		if cell.Rating != nil {
			m["rating"] = proficiencyRatingToJSON(cell.Rating)
		}
		cells[i] = m
	}
	return c.JSON(fiber.Map{
		"course_id":   gb.CourseID,
		"proficiency": proficiencyToJSON(gb.Proficiency),
		"students":    students,
		"outcomes":    outcomes,
		"cells":       cells,
	})
}
