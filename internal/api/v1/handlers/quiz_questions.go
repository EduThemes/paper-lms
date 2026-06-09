package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizQuestionHandler struct {
	quizService *service.QuizService
}

func NewQuizQuestionHandler(quizService *service.QuizService) *QuizQuestionHandler {
	return &QuizQuestionHandler{quizService: quizService}
}

func quizQuestionToJSON(q *models.QuizQuestion) fiber.Map {
	return fiber.Map{
		"id":                      q.ID,
		"quiz_id":                 q.QuizID,
		"quiz_question_group_id":  q.QuizQuestionGroupID,
		"position":                q.Position,
		"question_type":           q.QuestionType,
		"question_text":           q.QuestionText,
		"points_possible":         q.PointsPossible,
		"answers":                 q.Answers,
		"correct_comments":        q.CorrectComments,
		"incorrect_comments":      q.IncorrectComments,
		"neutral_comments":        q.NeutralComments,
		"workflow_state":          q.WorkflowState,
		"bank_item_id":            q.BankItemID,
		"stimulus_id":             q.StimulusID,
		"created_at":              q.CreatedAt,
		"updated_at":              q.UpdatedAt,
	}
}

// scopedQuizQuestion loads the :question_id question and verifies it
// belongs to :quiz_id, which must itself live in :course_id and the
// caller's tenant. Returns wrote=true (a 404 has been written) on any
// miss so the caller short-circuits with `return nil`. Closes the
// nested-route cross-tenant / cross-quiz IDOR — the route middleware
// only guards :course_id.
func (h *QuizQuestionHandler) scopedQuizQuestion(c *fiber.Ctx) (*models.QuizQuestion, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid quiz ID")
		return nil, true
	}
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid question ID")
		return nil, true
	}
	if requireQuizInCourse(c, h.quizService, uint(quizID), uint(courseID)) {
		return nil, true
	}
	question, err := h.quizService.GetQuestion(c.Context(), uint(questionID))
	if err != nil || question.QuizID != uint(quizID) {
		_ = responses.NotFound(c, "quiz question")
		return nil, true
	}
	return question, false
}

func (h *QuizQuestionHandler) ListQuestions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if requireQuizInCourse(c, h.quizService, uint(quizID), uint(courseID)) {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.quizService.ListQuestions(c.Context(), uint(quizID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch quiz questions")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	questions := make([]fiber.Map, len(result.Items))
	for i, q := range result.Items {
		questions[i] = quizQuestionToJSON(&q)
	}

	return c.JSON(questions)
}

func (h *QuizQuestionHandler) GetQuestion(c *fiber.Ctx) error {
	question, wrote := h.scopedQuizQuestion(c)
	if wrote {
		return nil
	}

	return c.JSON(quizQuestionToJSON(question))
}

func (h *QuizQuestionHandler) CreateQuestion(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if requireQuizInCourse(c, h.quizService, uint(quizID), uint(courseID)) {
		return nil
	}

	var input struct {
		Question struct {
			Position            int      `json:"position"`
			QuestionType        string   `json:"question_type"`
			QuestionText        string   `json:"question_text"`
			PointsPossible      *float64 `json:"points_possible"`
			Answers             string   `json:"answers"`
			CorrectComments     string   `json:"correct_comments"`
			IncorrectComments   string   `json:"incorrect_comments"`
			NeutralComments     string   `json:"neutral_comments"`
			QuizQuestionGroupID *uint    `json:"quiz_question_group_id"`
		} `json:"question"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	question := &models.QuizQuestion{
		QuizID:              uint(quizID),
		QuizQuestionGroupID: input.Question.QuizQuestionGroupID,
		Position:            input.Question.Position,
		QuestionType:        input.Question.QuestionType,
		QuestionText:        service.SanitizeHTML(input.Question.QuestionText),
		PointsPossible:      input.Question.PointsPossible,
		Answers:             input.Question.Answers,
		CorrectComments:     input.Question.CorrectComments,
		IncorrectComments:   input.Question.IncorrectComments,
		NeutralComments:     input.Question.NeutralComments,
	}

	if err := h.quizService.CreateQuestion(c.Context(), question); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(quizQuestionToJSON(question))
}

func (h *QuizQuestionHandler) UpdateQuestion(c *fiber.Ctx) error {
	question, wrote := h.scopedQuizQuestion(c)
	if wrote {
		return nil
	}

	var input struct {
		Question struct {
			Position            *int     `json:"position"`
			QuestionType        *string  `json:"question_type"`
			QuestionText        *string  `json:"question_text"`
			PointsPossible      *float64 `json:"points_possible"`
			Answers             *string  `json:"answers"`
			CorrectComments     *string  `json:"correct_comments"`
			IncorrectComments   *string  `json:"incorrect_comments"`
			NeutralComments     *string  `json:"neutral_comments"`
			QuizQuestionGroupID *uint    `json:"quiz_question_group_id"`
		} `json:"question"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Question.Position != nil {
		question.Position = *input.Question.Position
	}
	if input.Question.QuestionType != nil {
		question.QuestionType = *input.Question.QuestionType
	}
	if input.Question.QuestionText != nil {
		question.QuestionText = service.SanitizeHTML(*input.Question.QuestionText)
	}
	if input.Question.PointsPossible != nil {
		question.PointsPossible = input.Question.PointsPossible
	}
	if input.Question.Answers != nil {
		question.Answers = *input.Question.Answers
	}
	if input.Question.CorrectComments != nil {
		question.CorrectComments = *input.Question.CorrectComments
	}
	if input.Question.IncorrectComments != nil {
		question.IncorrectComments = *input.Question.IncorrectComments
	}
	if input.Question.NeutralComments != nil {
		question.NeutralComments = *input.Question.NeutralComments
	}
	if input.Question.QuizQuestionGroupID != nil {
		question.QuizQuestionGroupID = input.Question.QuizQuestionGroupID
	}

	if err := h.quizService.UpdateQuestion(c.Context(), question); err != nil {
		return responses.InternalError(c, "Could not update quiz question")
	}

	return c.JSON(quizQuestionToJSON(question))
}

func (h *QuizQuestionHandler) DeleteQuestion(c *fiber.Ctx) error {
	question, wrote := h.scopedQuizQuestion(c)
	if wrote {
		return nil
	}

	if err := h.quizService.DeleteQuestion(c.Context(), question.ID); err != nil {
		return responses.InternalError(c, "Could not delete quiz question")
	}

	return c.JSON(fiber.Map{"delete": true})
}
