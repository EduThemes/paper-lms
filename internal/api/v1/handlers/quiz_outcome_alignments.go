package handlers

import (
	"errors"

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

// alignmentQuestionScope parses :course_id/:quiz_id/:question_id and ties
// question -> quiz -> course in the caller's tenant. Any miss is a 404.
func (h *QuizOutcomeAlignmentHandler) alignmentQuestionScope(c *fiber.Ctx) (courseID, quizID, questionID uint, err error) {
	cid, e1 := c.ParamsInt("course_id")
	qid, e2 := c.ParamsInt("quiz_id")
	qqid, e3 := c.ParamsInt("question_id")
	if e1 != nil || e2 != nil || e3 != nil || cid <= 0 || qid <= 0 || qqid <= 0 {
		return 0, 0, 0, fiber.NewError(fiber.StatusBadRequest, "Invalid course, quiz or question ID")
	}
	if err := h.svc.RequireQuestionInCourse(c.Context(), uint(cid), uint(qid), uint(qqid), callerAccountID(c)); err != nil {
		return 0, 0, 0, authzNotFound("quiz question")
	}
	return uint(cid), uint(qid), uint(qqid), nil
}

// Align creates an alignment. The question must belong to a quiz of the
// course, and the outcome must be the course's own or an account outcome of
// the caller's tenant.
// POST /api/v1/courses/:course_id/quizzes/:quiz_id/questions/:question_id/outcome_alignments
//
//	body: {"outcome_id": N, "mastery_threshold": 0.7}
func (h *QuizOutcomeAlignmentHandler) Align(c *fiber.Ctx) error {
	courseID, quizID, questionID, err := h.alignmentQuestionScope(c)
	if err != nil {
		return err
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
	a, err := h.svc.AlignInCourse(c.Context(), courseID, quizID, questionID, input.OutcomeID, callerAccountID(c), threshold)
	if err != nil {
		if errors.Is(err, service.ErrAlignmentScope) {
			return authzNotFound("outcome")
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizOutcomeAlignmentToJSON(a))
}

// Unalign deletes an alignment.
// DELETE /api/v1/courses/:course_id/quizzes/:quiz_id/questions/:question_id/outcome_alignments/:outcome_id
func (h *QuizOutcomeAlignmentHandler) Unalign(c *fiber.Ctx) error {
	_, _, questionID, err := h.alignmentQuestionScope(c)
	if err != nil {
		return err
	}
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid outcome ID")
	}
	if err := h.svc.Unalign(c.Context(), questionID, uint(outcomeID)); err != nil {
		return responses.InternalError(c, "Could not remove alignment")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// ListByQuestion returns every alignment for a quiz question.
// GET /api/v1/courses/:course_id/quizzes/:quiz_id/questions/:question_id/outcome_alignments
func (h *QuizOutcomeAlignmentHandler) ListByQuestion(c *fiber.Ctx) error {
	_, _, questionID, err := h.alignmentQuestionScope(c)
	if err != nil {
		return err
	}
	items, err := h.svc.ListByQuestion(c.Context(), questionID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch alignments")
	}
	out := make([]fiber.Map, len(items))
	for i, a := range items {
		out[i] = quizOutcomeAlignmentToJSON(&a)
	}
	return c.JSON(out)
}

// ListByOutcome returns the outcome's alignments to questions in this course.
// GET /api/v1/courses/:course_id/outcomes/:outcome_id/quiz_question_alignments
func (h *QuizOutcomeAlignmentHandler) ListByOutcome(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	outcomeID, err := c.ParamsInt("outcome_id")
	if err != nil || outcomeID <= 0 {
		return responses.BadRequest(c, "Invalid outcome ID")
	}
	items, err := h.svc.ListByOutcomeInCourse(c.Context(), uint(courseID), uint(outcomeID), callerAccountID(c))
	if err != nil {
		if errors.Is(err, service.ErrAlignmentScope) {
			return authzNotFound("outcome")
		}
		return responses.InternalError(c, "Could not fetch alignments")
	}
	out := make([]fiber.Map, len(items))
	for i, a := range items {
		out[i] = quizOutcomeAlignmentToJSON(&a)
	}
	return c.JSON(out)
}
