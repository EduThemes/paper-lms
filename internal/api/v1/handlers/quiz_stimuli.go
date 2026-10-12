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

// stimulusQuestionParams parses :course_id / :stimulus_id (and
// :question_id when wantQuestion). Writes a 400 and returns ok=false on a
// malformed id.
func stimulusQuestionParams(c *fiber.Ctx, wantQuestion bool) (courseID, stimulusID, questionID uint, ok bool) {
	cid, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return 0, 0, 0, false
	}
	sid, err := c.ParamsInt("stimulus_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid stimulus ID")
		return 0, 0, 0, false
	}
	if wantQuestion {
		qid, err := c.ParamsInt("question_id")
		if err != nil {
			_ = responses.BadRequest(c, "Invalid question ID")
			return 0, 0, 0, false
		}
		questionID = uint(qid)
	}
	return uint(cid), uint(sid), questionID, true
}

// LinkQuestion attaches a stimulus to a quiz question. Both must belong to
// :course_id (B126 / answer-key exposure: these routes used to be
// course-less and unscoped).
// POST /api/v1/courses/:course_id/quiz_stimuli/:stimulus_id/questions/:question_id
func (h *QuizStimulusHandler) LinkQuestion(c *fiber.Ctx) error {
	courseID, stimulusID, questionID, ok := stimulusQuestionParams(c, true)
	if !ok {
		return nil
	}
	if err := h.svc.LinkQuestionToStimulusInCourse(c.Context(), courseID, callerAccountID(c), questionID, stimulusID); err != nil {
		return responses.NotFound(c, "quiz question")
	}
	return c.JSON(fiber.Map{"linked": true})
}

// UnlinkQuestion clears the stimulus link on a question of :course_id.
// DELETE /api/v1/courses/:course_id/quiz_stimuli/:stimulus_id/questions/:question_id
func (h *QuizStimulusHandler) UnlinkQuestion(c *fiber.Ctx) error {
	courseID, stimulusID, questionID, ok := stimulusQuestionParams(c, true)
	if !ok {
		return nil
	}
	if err := h.svc.UnlinkQuestionFromStimulusInCourse(c.Context(), courseID, callerAccountID(c), questionID, stimulusID); err != nil {
		return responses.NotFound(c, "quiz question")
	}
	return c.JSON(fiber.Map{"unlinked": true})
}

// ListQuestions returns the course's quiz questions pointing at this
// stimulus — WITH answer keys, so the route is instructor-only.
// GET /api/v1/courses/:course_id/quiz_stimuli/:stimulus_id/questions
func (h *QuizStimulusHandler) ListQuestions(c *fiber.Ctx) error {
	courseID, stimulusID, _, ok := stimulusQuestionParams(c, false)
	if !ok {
		return nil
	}
	if !quizCallerIsStaff(c) {
		return responses.NotFound(c, "stimulus")
	}
	questions, err := h.svc.ListQuestionsForStimulusInCourse(c.Context(), courseID, callerAccountID(c), stimulusID)
	if err != nil {
		return responses.NotFound(c, "stimulus")
	}
	out := make([]fiber.Map, len(questions))
	for i := range questions {
		out[i] = quizQuestionToJSON(&questions[i])
	}
	return c.JSON(out)
}
