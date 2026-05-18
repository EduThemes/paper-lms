package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizSubmissionHandler struct {
	quizService     *service.QuizService
	observerService *service.ObserverService
	auditService    *service.AuditService
}

func NewQuizSubmissionHandler(quizService *service.QuizService, observerService *service.ObserverService, auditService *service.AuditService) *QuizSubmissionHandler {
	return &QuizSubmissionHandler{quizService: quizService, observerService: observerService, auditService: auditService}
}

func quizSubmissionToJSON(qs *models.QuizSubmission) fiber.Map {
	m := fiber.Map{
		"id":               qs.ID,
		"quiz_id":          qs.QuizID,
		"user_id":          qs.UserID,
		"submission_id":    qs.SubmissionID,
		"attempt":          qs.Attempt,
		"score":            qs.Score,
		"kept_score":       qs.KeptScore,
		"started_at":       qs.StartedAt,
		"finished_at":      qs.FinishedAt,
		"end_at":           qs.EndAt,
		"time_spent":       qs.TimeSpent,
		"workflow_state":   qs.WorkflowState,
		"validation_token": qs.ValidationToken,
		"created_at":       qs.CreatedAt,
		"updated_at":       qs.UpdatedAt,
	}
	if qs.SelectedQuestions != "" {
		m["selected_questions"] = qs.SelectedQuestions
	}
	return m
}

func quizSubmissionAnswerToJSON(a *models.QuizSubmissionAnswer) fiber.Map {
	return fiber.Map{
		"id":                 a.ID,
		"quiz_submission_id": a.QuizSubmissionID,
		"question_id":        a.QuestionID,
		"answer":             a.Answer,
		"correct":            a.Correct,
		"points":             a.Points,
		"created_at":         a.CreatedAt,
		"updated_at":         a.UpdatedAt,
	}
}

// StartSubmission handles POST /courses/:course_id/quizzes/:quiz_id/submissions
func (h *QuizSubmissionHandler) StartSubmission(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		TimeLimit *int `json:"time_limit"` // optional override in minutes
	}
	// Body is optional for starting a submission
	_ = c.BodyParser(&input)

	submission, err := h.quizService.StartSubmission(c.Context(), uint(quizID), userID, input.TimeLimit)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"quiz_submissions": []fiber.Map{quizSubmissionToJSON(submission)},
	})
}

// GetSubmission handles GET /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id
func (h *QuizSubmissionHandler) GetSubmission(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	submissionID, err := c.ParamsInt("submission_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid submission ID")
	}

	submission, err := h.quizService.GetSubmission(c.Context(), uint(submissionID))
	if err != nil {
		return responses.NotFound(c, "quiz submission")
	}

	// Verify submission belongs to the URL's quiz (prevents cross-course IDOR)
	if submission.QuizID != uint(quizID) {
		return responses.NotFound(c, "quiz submission")
	}

	// Authorization: only the submission owner, instructor, or observer can view it
	userID, _ := c.Locals("user_id").(uint)
	if submission.UserID != userID {
		enrollmentType, _ := c.Locals("enrollment_type").(string)
		isTeacherOrTA := enrollmentType == "TeacherEnrollment" || enrollmentType == "TaEnrollment"
		isObserver := false
		if h.observerService != nil {
			isObserver, _ = h.observerService.IsObserverOf(c.Context(), userID, submission.UserID)
		}
		if !isTeacherOrTA && !isObserver {
			// 13.1.E: existence leak — return 404 not 403. The
			// submission exists but belongs to a different student in a
			// course where the caller has no role; 403 confirms the
			// row exists.
			return responses.NotFound(c, "quiz submission")
		}
	}

	// 13.5 PII audit — single-student read of a quiz attempt.
	if userID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), userID, submission.UserID, "read", "quiz_attempt", "quiz_submissions", submission.ID, c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"quiz_submissions": []fiber.Map{quizSubmissionToJSON(submission)},
	})
}

// AnswerQuestion handles PUT /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id
func (h *QuizSubmissionHandler) AnswerQuestion(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	submissionID, err := c.ParamsInt("submission_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid submission ID")
	}

	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}

	// Authorization: only the submission owner can answer questions
	userID, _ := c.Locals("user_id").(uint)
	submission, err := h.quizService.GetSubmission(c.Context(), uint(submissionID))
	if err != nil {
		return responses.NotFound(c, "quiz submission")
	}
	// Verify submission belongs to the URL's quiz
	if submission.QuizID != uint(quizID) {
		return responses.NotFound(c, "quiz submission")
	}
	if submission.UserID != userID {
		// 13.1.E: existence leak — return 404 not 403. The submission
		// exists but belongs to another student; 403 confirms existence.
		return responses.NotFound(c, "quiz submission")
	}

	var input struct {
		Answer string `json:"answer"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	answer, err := h.quizService.AnswerQuestion(c.Context(), uint(submissionID), uint(questionID), input.Answer)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(quizSubmissionAnswerToJSON(answer))
}

// CompleteSubmission handles POST /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/complete
func (h *QuizSubmissionHandler) CompleteSubmission(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	submissionID, err := c.ParamsInt("submission_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid submission ID")
	}

	// Verify submission belongs to the URL's quiz
	sub, err := h.quizService.GetSubmission(c.Context(), uint(submissionID))
	if err != nil {
		return responses.NotFound(c, "quiz submission")
	}
	if sub.QuizID != uint(quizID) {
		return responses.NotFound(c, "quiz submission")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	submission, err := h.quizService.CompleteSubmission(c.Context(), uint(submissionID), userID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"quiz_submissions": []fiber.Map{quizSubmissionToJSON(submission)},
	})
}

// GetSubmissionAnswers handles GET /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/answers
func (h *QuizSubmissionHandler) GetSubmissionAnswers(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	submissionID, err := c.ParamsInt("submission_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid submission ID")
	}

	// Verify submission belongs to the URL's quiz
	sub, err := h.quizService.GetSubmission(c.Context(), uint(submissionID))
	if err != nil {
		return responses.NotFound(c, "quiz submission")
	}
	if sub.QuizID != uint(quizID) {
		return responses.NotFound(c, "quiz submission")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	answers, err := h.quizService.ListSubmissionAnswers(c.Context(), uint(submissionID), userID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	result := make([]fiber.Map, len(answers))
	for i, a := range answers {
		result[i] = quizSubmissionAnswerToJSON(&a)
	}

	// 13.5 PII audit — student-keyed read of a quiz attempt's answers.
	// Subject is the quiz submission's owner (sub.UserID).
	if userID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), userID, sub.UserID, "read", "quiz_answers", "quiz_submissions", sub.ID, c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"quiz_submission_answers": result,
	})
}

// GetSubmissionQuestions handles GET /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions
// Returns the personalized set of questions for this submission (randomized from groups).
func (h *QuizSubmissionHandler) GetSubmissionQuestions(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	submissionID, err := c.ParamsInt("submission_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid submission ID")
	}

	// Verify submission exists and belongs to this quiz
	submission, err := h.quizService.GetSubmission(c.Context(), uint(submissionID))
	if err != nil {
		return responses.NotFound(c, "quiz submission")
	}
	if submission.QuizID != uint(quizID) {
		return responses.NotFound(c, "quiz submission")
	}

	// Authorization: only the submission owner, instructor, or observer can view
	userID, _ := c.Locals("user_id").(uint)
	if submission.UserID != userID {
		enrollmentType, _ := c.Locals("enrollment_type").(string)
		isTeacherOrTA := enrollmentType == "TeacherEnrollment" || enrollmentType == "TaEnrollment"
		isObserver := false
		if h.observerService != nil {
			isObserver, _ = h.observerService.IsObserverOf(c.Context(), userID, submission.UserID)
		}
		if !isTeacherOrTA && !isObserver {
			// 13.1.E: existence leak — return 404 not 403. Same
			// rationale as GetSubmission above.
			return responses.NotFound(c, "quiz submission")
		}
	}

	questions, err := h.quizService.GetSubmissionQuestions(c.Context(), uint(submissionID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch submission questions")
	}

	result := make([]fiber.Map, len(questions))
	for i, q := range questions {
		result[i] = quizQuestionToJSON(&q)
	}

	return c.JSON(result)
}

// ListSubmissions handles GET /courses/:course_id/quizzes/:quiz_id/submissions
func (h *QuizSubmissionHandler) ListSubmissions(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.quizService.ListSubmissions(c.Context(), uint(quizID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch quiz submissions")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	submissions := make([]fiber.Map, len(result.Items))
	for i, qs := range result.Items {
		submissions[i] = quizSubmissionToJSON(&qs)
	}

	// 13.5 PII audit — bulk-read semantics. Per-row would explode the
	// audit table on large quiz rosters; one row per call with
	// student_id=0 + data_field="bulk_quiz_attempt_list_read" is what
	// audit investigations actually want (who pulled this quiz's
	// attempts, when).
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_quiz_attempt_list_read", "quiz_submissions", uint(quizID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"quiz_submissions": submissions,
	})
}
