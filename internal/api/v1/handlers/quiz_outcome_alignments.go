package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizOutcomeAlignmentHandler struct {
	svc *service.QuizOutcomeAlignmentService
}

func NewQuizOutcomeAlignmentHandler(svc *service.QuizOutcomeAlignmentService) *QuizOutcomeAlignmentHandler {
	return &QuizOutcomeAlignmentHandler{svc: svc}
}

func quizOutcomeAlignmentToJSON(a *models.QuizQuestionOutcomeAlignment) fiber.Map {
	return fiber.Map{
		"id":                a.ID,
		"quiz_question_id":  a.QuizQuestionID,
		"outcome_id":        a.OutcomeID,
		"mastery_threshold": a.MasteryThreshold,
		"created_at":        a.CreatedAt,
	}
}

// Align creates an alignment.
// POST /api/v1/quiz_questions/:question_id/outcome_alignments
//
//	body: {"outcome_id": N, "mastery_threshold": 0.7}
func (h *QuizOutcomeAlignmentHandler) Align(c *fiber.Ctx) error {
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	var input struct {
		OutcomeID        uint     `json:"outcome_id"`
		MasteryThreshold *float64 `json:"mastery_threshold"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	threshold := 0.7
	if input.MasteryThreshold != nil {
		threshold = *input.MasteryThreshold
	}
	a, err := h.svc.Align(c.Context(), uint(questionID), input.OutcomeID, threshold)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizOutcomeAlignmentToJSON(a))
}

// Unalign deletes an alignment.
// DELETE /api/v1/quiz_questions/:question_id/outcome_alignments/:outcome_id
func (h *QuizOutcomeAlignmentHandler) Unalign(c *fiber.Ctx) error {
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}
	if err := h.svc.Unalign(c.Context(), uint(questionID), uint(outcomeID)); err != nil {
		return responses.InternalError(c, "Could not remove alignment")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// ListByQuestion returns every alignment for a quiz question.
// GET /api/v1/quiz_questions/:question_id/outcome_alignments
func (h *QuizOutcomeAlignmentHandler) ListByQuestion(c *fiber.Ctx) error {
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	items, err := h.svc.ListByQuestion(c.Context(), uint(questionID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch alignments")
	}
	out := make([]fiber.Map, len(items))
	for i, a := range items {
		out[i] = quizOutcomeAlignmentToJSON(&a)
	}
	return c.JSON(out)
}

// ListByOutcome returns every alignment for a learning outcome.
// GET /api/v1/learning_outcomes/:outcome_id/quiz_question_alignments
func (h *QuizOutcomeAlignmentHandler) ListByOutcome(c *fiber.Ctx) error {
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}
	items, err := h.svc.ListByOutcome(c.Context(), uint(outcomeID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch alignments")
	}
	out := make([]fiber.Map, len(items))
	for i, a := range items {
		out[i] = quizOutcomeAlignmentToJSON(&a)
	}
	return c.JSON(out)
}
