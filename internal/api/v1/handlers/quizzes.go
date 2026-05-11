package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type QuizHandler struct {
	quizRepo repository.QuizRepository
}

func NewQuizHandler(quizRepo repository.QuizRepository) *QuizHandler {
	return &QuizHandler{quizRepo: quizRepo}
}

func quizToJSON(q *models.Quiz) fiber.Map {
	return fiber.Map{
		"id":               q.ID,
		"course_id":        q.CourseID,
		"title":            q.Title,
		"description":      q.Description,
		"quiz_type":        q.QuizType,
		"time_limit":       q.TimeLimit,
		"allowed_attempts": q.AllowedAttempts,
		"due_at":           q.DueAt,
		"unlock_at":        q.UnlockAt,
		"lock_at":          q.LockAt,
		"points_possible":  q.PointsPossible,
		"published":        q.Published,
		"workflow_state":   q.WorkflowState,
		"created_at":       q.CreatedAt,
		"updated_at":       q.UpdatedAt,
	}
}

func (h *QuizHandler) ListQuizzes(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	result, err := h.quizRepo.ListByCourseID(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch quizzes")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	quizzes := make([]fiber.Map, len(result.Items))
	for i, q := range result.Items {
		quizzes[i] = quizToJSON(&q)
	}
	return c.JSON(quizzes)
}

func (h *QuizHandler) GetQuiz(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	quiz, err := h.quizRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "quiz")
	}

	return c.JSON(quizToJSON(quiz))
}

func (h *QuizHandler) CreateQuiz(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Quiz struct {
			Title           string     `json:"title"`
			Description     string     `json:"description"`
			QuizType        string     `json:"quiz_type"`
			TimeLimit       *int       `json:"time_limit"`
			AllowedAttempts int        `json:"allowed_attempts"`
			DueAt           *time.Time `json:"due_at"`
			UnlockAt        *time.Time `json:"unlock_at"`
			LockAt          *time.Time `json:"lock_at"`
			PointsPossible  *float64   `json:"points_possible"`
			Published       bool       `json:"published"`
		} `json:"quiz"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Quiz.Title == "" {
		return responses.BadRequest(c, "Quiz title is required")
	}

	state := "unpublished"
	if input.Quiz.Published {
		state = "published"
	}

	quiz := &models.Quiz{
		CourseID:        uint(courseID),
		Title:           input.Quiz.Title,
		Description:     input.Quiz.Description,
		QuizType:        input.Quiz.QuizType,
		TimeLimit:       input.Quiz.TimeLimit,
		AllowedAttempts: input.Quiz.AllowedAttempts,
		DueAt:           input.Quiz.DueAt,
		UnlockAt:        input.Quiz.UnlockAt,
		LockAt:          input.Quiz.LockAt,
		PointsPossible:  input.Quiz.PointsPossible,
		Published:       input.Quiz.Published,
		WorkflowState:   state,
	}

	if quiz.QuizType == "" {
		quiz.QuizType = "assignment"
	}
	if quiz.AllowedAttempts == 0 {
		quiz.AllowedAttempts = 1
	}

	if err := h.quizRepo.Create(c.Context(), quiz); err != nil {
		return responses.InternalError(c, "Could not create quiz")
	}

	return c.Status(fiber.StatusCreated).JSON(quizToJSON(quiz))
}

func (h *QuizHandler) UpdateQuiz(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	quiz, err := h.quizRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "quiz")
	}

	var input struct {
		Quiz struct {
			Title           *string    `json:"title"`
			Description     *string    `json:"description"`
			QuizType        *string    `json:"quiz_type"`
			TimeLimit       *int       `json:"time_limit"`
			AllowedAttempts *int       `json:"allowed_attempts"`
			DueAt           *time.Time `json:"due_at"`
			UnlockAt        *time.Time `json:"unlock_at"`
			LockAt          *time.Time `json:"lock_at"`
			PointsPossible  *float64   `json:"points_possible"`
			Published       *bool      `json:"published"`
		} `json:"quiz"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Quiz.Title != nil {
		quiz.Title = *input.Quiz.Title
	}
	if input.Quiz.Description != nil {
		quiz.Description = *input.Quiz.Description
	}
	if input.Quiz.QuizType != nil {
		quiz.QuizType = *input.Quiz.QuizType
	}
	if input.Quiz.TimeLimit != nil {
		quiz.TimeLimit = input.Quiz.TimeLimit
	}
	if input.Quiz.AllowedAttempts != nil {
		quiz.AllowedAttempts = *input.Quiz.AllowedAttempts
	}
	if input.Quiz.DueAt != nil {
		quiz.DueAt = input.Quiz.DueAt
	}
	if input.Quiz.UnlockAt != nil {
		quiz.UnlockAt = input.Quiz.UnlockAt
	}
	if input.Quiz.LockAt != nil {
		quiz.LockAt = input.Quiz.LockAt
	}
	if input.Quiz.PointsPossible != nil {
		quiz.PointsPossible = input.Quiz.PointsPossible
	}
	if input.Quiz.Published != nil {
		quiz.Published = *input.Quiz.Published
		if *input.Quiz.Published {
			quiz.WorkflowState = "published"
		} else {
			quiz.WorkflowState = "unpublished"
		}
	}

	if err := h.quizRepo.Update(c.Context(), quiz); err != nil {
		return responses.InternalError(c, "Could not update quiz")
	}

	return c.JSON(quizToJSON(quiz))
}

func (h *QuizHandler) DeleteQuiz(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	if err := h.quizRepo.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete quiz")
	}

	return c.JSON(fiber.Map{"delete": true})
}
