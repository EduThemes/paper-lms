package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizHandler struct {
	quizRepo repository.QuizRepository
}

func NewQuizHandler(quizRepo repository.QuizRepository) *QuizHandler {
	return &QuizHandler{quizRepo: quizRepo}
}

func quizToJSON(q *models.Quiz) fiber.Map {
	m := fiber.Map{
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
	return addQuizDeliveryJSON(m, q)
}

func (h *QuizHandler) ListQuizzes(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	// B128: students (and observers) only ever see published quizzes.
	list := h.quizRepo.ListPublishedByCourseID
	if quizCallerIsStaff(c) {
		list = h.quizRepo.ListByCourseID
	}
	result, err := list(c.Context(), uint(courseID), params)
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
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	quiz, err := h.quizRepo.FindByID(c.Context(), uint(id), callerAccountID(c))
	// F-013 parent-course tie; deleted quizzes are gone; non-staff callers
	// cannot see unpublished quizzes (B128).
	if err != nil || quiz.CourseID != uint(courseID) || quiz.WorkflowState == "deleted" ||
		(!quizCallerIsStaff(c) && !quizVisibleToStudents(quiz)) {
		return responses.NotFound(c, "quiz")
	}

	return c.JSON(quizToViewerJSON(c, quiz))
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
		Description:     service.SanitizeHTML(input.Quiz.Description),
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
	if applyQuizDeliveryInput(c, quiz) {
		return nil
	}

	if err := h.quizRepo.Create(c.Context(), quiz); err != nil {
		return responses.InternalError(c, "Could not create quiz")
	}

	return c.Status(fiber.StatusCreated).JSON(quizToViewerJSON(c, quiz))
}

func (h *QuizHandler) UpdateQuiz(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	quiz, err := h.quizRepo.FindByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "quiz")
	}
	// F-013: parent-course tie.
	if quiz.CourseID != uint(courseID) {
		return responses.NotFound(c, "quiz")
	}
	// A soft-deleted quiz is terminal — FindByID does not exclude deleted
	// rows, so without this guard a PUT {"published":true} would resurrect
	// it to `published`. Treat a deleted quiz as gone.
	if quiz.WorkflowState == "deleted" {
		return responses.NotFound(c, "quiz")
	}

	var input struct {
		Quiz struct {
			Title           *string    `json:"title"`
			Description     *string    `json:"description"`
			QuizType        *string    `json:"quiz_type"`
			// optionalField: an explicit null clears the value (B129);
			// an absent key leaves it unchanged.
			TimeLimit       optionalField[int]       `json:"time_limit"`
			AllowedAttempts *int                     `json:"allowed_attempts"`
			DueAt           optionalField[time.Time] `json:"due_at"`
			UnlockAt        optionalField[time.Time] `json:"unlock_at"`
			LockAt          optionalField[time.Time] `json:"lock_at"`
			PointsPossible  *float64                 `json:"points_possible"`
			Published       *bool                    `json:"published"`
		} `json:"quiz"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Quiz.Title != nil {
		quiz.Title = *input.Quiz.Title
	}
	if input.Quiz.Description != nil {
		quiz.Description = service.SanitizeHTML(*input.Quiz.Description)
	}
	if input.Quiz.QuizType != nil {
		quiz.QuizType = *input.Quiz.QuizType
	}
	if input.Quiz.TimeLimit.Set {
		if input.Quiz.TimeLimit.Null || input.Quiz.TimeLimit.Value <= 0 {
			quiz.TimeLimit = nil
		} else {
			v := input.Quiz.TimeLimit.Value
			quiz.TimeLimit = &v
		}
	}
	if input.Quiz.AllowedAttempts != nil {
		quiz.AllowedAttempts = *input.Quiz.AllowedAttempts
	}
	applyOptionalTime(&quiz.DueAt, input.Quiz.DueAt)
	applyOptionalTime(&quiz.UnlockAt, input.Quiz.UnlockAt)
	applyOptionalTime(&quiz.LockAt, input.Quiz.LockAt)
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

	if applyQuizDeliveryInput(c, quiz) {
		return nil
	}

	if err := h.quizRepo.Update(c.Context(), quiz); err != nil {
		return responses.InternalError(c, "Could not update quiz")
	}

	return c.JSON(quizToViewerJSON(c, quiz))
}

// applyOptionalTime sets *dst from a PATCH-style optional field: null
// clears it, a value replaces it, absence leaves it unchanged.
func applyOptionalTime(dst **time.Time, f optionalField[time.Time]) {
	if !f.Set {
		return
	}
	if f.Null {
		*dst = nil
		return
	}
	v := f.Value
	*dst = &v
}

func (h *QuizHandler) DeleteQuiz(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	// F-011 / F-013: tenant + parent-course tie before destructive op.
	quiz, err := h.quizRepo.FindByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "quiz")
	}
	if quiz.CourseID != uint(courseID) {
		return responses.NotFound(c, "quiz")
	}

	if err := h.quizRepo.Delete(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete quiz")
	}

	return c.JSON(fiber.Map{"delete": true})
}
