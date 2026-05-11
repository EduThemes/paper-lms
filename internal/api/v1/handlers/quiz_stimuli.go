package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizStimulusHandler struct {
	svc *service.QuizStimulusService
}

func NewQuizStimulusHandler(svc *service.QuizStimulusService) *QuizStimulusHandler {
	return &QuizStimulusHandler{svc: svc}
}

func quizStimulusToJSON(s *models.QuizStimulus) fiber.Map {
	return fiber.Map{
		"id":         s.ID,
		"course_id":  s.CourseID,
		"title":      s.Title,
		"content":    s.Content,
		"created_at": s.CreatedAt,
		"updated_at": s.UpdatedAt,
	}
}

func (h *QuizStimulusHandler) ListStimuli(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	params := middleware.GetPagination(c)
	result, err := h.svc.ListStimuli(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch stimuli")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
	out := make([]fiber.Map, len(result.Items))
	for i, s := range result.Items {
		out[i] = quizStimulusToJSON(&s)
	}
	return c.JSON(out)
}

func (h *QuizStimulusHandler) GetStimulus(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("stimulus_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid stimulus ID")
	}
	stim, err := h.svc.GetStimulus(c.Context(), uint(courseID), uint(id))
	if err != nil {
		return responses.NotFound(c, "stimulus")
	}
	return c.JSON(quizStimulusToJSON(stim))
}

func (h *QuizStimulusHandler) CreateStimulus(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		Stimulus struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		} `json:"stimulus"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	stim := &models.QuizStimulus{
		CourseID: uint(courseID),
		Title:    input.Stimulus.Title,
		Content:  input.Stimulus.Content,
	}
	if err := h.svc.CreateStimulus(c.Context(), stim); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizStimulusToJSON(stim))
}

func (h *QuizStimulusHandler) UpdateStimulus(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("stimulus_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid stimulus ID")
	}
	stim, err := h.svc.GetStimulus(c.Context(), uint(courseID), uint(id))
	if err != nil {
		return responses.NotFound(c, "stimulus")
	}
	var input struct {
		Stimulus struct {
			Title   *string `json:"title"`
			Content *string `json:"content"`
		} `json:"stimulus"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Stimulus.Title != nil {
		stim.Title = *input.Stimulus.Title
	}
	if input.Stimulus.Content != nil {
		stim.Content = *input.Stimulus.Content
	}
	if err := h.svc.UpdateStimulus(c.Context(), uint(courseID), stim); err != nil {
		return responses.InternalError(c, "Could not update stimulus")
	}
	return c.JSON(quizStimulusToJSON(stim))
}

func (h *QuizStimulusHandler) DeleteStimulus(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("stimulus_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid stimulus ID")
	}
	if err := h.svc.DeleteStimulus(c.Context(), uint(courseID), uint(id)); err != nil {
		return responses.NotFound(c, "stimulus")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// LinkQuestion attaches a stimulus to a quiz question.
// POST /api/v1/quiz_stimuli/:stimulus_id/questions/:question_id
func (h *QuizStimulusHandler) LinkQuestion(c *fiber.Ctx) error {
	stimulusID, err := c.ParamsInt("stimulus_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid stimulus ID")
	}
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	if err := h.svc.LinkQuestionToStimulus(c.Context(), uint(questionID), uint(stimulusID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"linked": true})
}

func (h *QuizStimulusHandler) UnlinkQuestion(c *fiber.Ctx) error {
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	if err := h.svc.UnlinkQuestionFromStimulus(c.Context(), uint(questionID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"unlinked": true})
}

// ListQuestions returns every quiz_question pointing at this stimulus.
// GET /api/v1/quiz_stimuli/:stimulus_id/questions
func (h *QuizStimulusHandler) ListQuestions(c *fiber.Ctx) error {
	stimulusID, err := c.ParamsInt("stimulus_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid stimulus ID")
	}
	questions, err := h.svc.ListQuestionsForStimulus(c.Context(), uint(stimulusID))
	if err != nil {
		return responses.NotFound(c, "stimulus")
	}
	out := make([]fiber.Map, len(questions))
	for i, q := range questions {
		out[i] = quizQuestionToJSON(&q)
	}
	return c.JSON(out)
}
