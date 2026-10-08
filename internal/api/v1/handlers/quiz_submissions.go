package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type QuizSubmissionHandler struct {
	quizService     *service.QuizService
	observerService *service.ObserverService
	auditService    *service.AuditService
	fileService     *service.FileService
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
		"fudge_points":     qs.FudgePoints,
		// score_before_regrade: the attempt's score before a regrade moved it.
		"score_before_regrade": qs.ScoreBeforeRegrade,
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
		"graded_via":         a.GradedVia,
		"comment":            a.Comment,
		"created_at":         a.CreatedAt,
		"updated_at":         a.UpdatedAt,
	}
}

// quizSubmissionForViewer serializes an attempt for the caller. Staff see
// everything; a student or observer sees the score only while the quiz's
// gradebook grade is visible to the student (post policy).
func (h *QuizSubmissionHandler) quizSubmissionForViewer(c *fiber.Ctx, quiz *models.Quiz, qs *models.QuizSubmission) fiber.Map {
	m := quizSubmissionToJSON(qs)
	if quizCallerIsStaff(c) || quiz == nil || !qs.IsCompleted() {
		return m
	}
	if !h.quizService.StudentResultsView(c.Context(), quiz, qs, time.Now()).Score {
		m["score"] = nil
		m["kept_score"] = nil
		m["fudge_points"] = nil
		m["score_before_regrade"] = nil
		m["score_hidden"] = true
	}
	return m
}

// requireQuizInCourse verifies the URL's quiz_id is a real quiz inside
// the URL's course_id AND inside the caller's tenant. Used by every
// quiz-submission endpoint to close F-003 (cross-course teacher reads)
// and F-004 (cross-tenant submission enumeration). Non-staff callers
// additionally only reach published, non-deleted quizzes (B128).
//
// Returns the quiz and wrote=true if the response has already been written
// (404 on quiz-not-in-course); caller short-circuits.
func (h *QuizSubmissionHandler) requireQuizInCourse(c *fiber.Ctx, quizID, courseID uint) (*models.Quiz, bool) {
	return loadQuizForCaller(c, h.quizService, quizID, courseID)
}

// callerCanViewAttempt reports whether the caller may read another user's
// quiz attempt: the owner, course staff, or a linked observer.
func (h *QuizSubmissionHandler) callerCanViewAttempt(c *fiber.Ctx, ownerID uint) bool {
	userID, _ := c.Locals("user_id").(uint)
	if userID != 0 && userID == ownerID {
		return true
	}
	if quizCallerIsStaff(c) {
		return true
	}
	if h.observerService != nil && userID != 0 {
		if ok, _ := h.observerService.IsObserverOf(c.Context(), userID, ownerID); ok {
			return true
		}
	}
	return false
}

// loadSubmission parses :submission_id, loads it, and ties it to :quiz_id.
// Returns wrote=true after writing an error response.
func (h *QuizSubmissionHandler) loadSubmission(c *fiber.Ctx, quizID uint) (*models.QuizSubmission, bool) {
	submissionID, err := c.ParamsInt("submission_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid submission ID")
		return nil, true
	}
	submission, err := h.quizService.GetSubmission(c.Context(), uint(submissionID))
	if err != nil || submission == nil || submission.QuizID != quizID {
		_ = responses.NotFound(c, "quiz submission")
		return nil, true
	}
	return submission, false
}

// StartSubmission handles POST /courses/:course_id/quizzes/:quiz_id/submissions.
//
// SECURITY (F-046): the pre-fix path read an optional `time_limit`
// from the request body and forwarded it directly to the service,
// which used it as the authoritative time limit when non-nil. A
// student could simply POST {"time_limit": 99999} (or 0 to disable)
// and trivially blow past the quiz's actual TimeLimit. There is no
// legitimate use of a student-controlled time-limit override. The
// quiz's own TimeLimit (plus any accommodations) is the only signal.
func (h *QuizSubmissionHandler) StartSubmission(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if _, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID)); wrote {
		return nil
	}

	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	submission, err := h.quizService.StartSubmission(c.Context(), uint(quizID), userID, nil /* time_limit is server-derived, not body-controlled */)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"quiz_submissions": []fiber.Map{quizSubmissionToJSON(submission)},
	})
}

// GetSubmission handles GET /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id
func (h *QuizSubmissionHandler) GetSubmission(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	quiz, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID))
	if wrote {
		return nil
	}

	submission, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}

	// Authorization: only the submission owner, instructor, or observer can view it.
	// 13.1.E: existence leak — return 404 not 403.
	if !h.callerCanViewAttempt(c, submission.UserID) {
		return responses.NotFound(c, "quiz submission")
	}

	// 13.5 PII audit — single-student read of a quiz attempt.
	userID, _ := c.Locals("user_id").(uint)
	if userID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), userID, submission.UserID, "read", "quiz_attempt", "quiz_submissions", submission.ID, c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"quiz_submissions": []fiber.Map{h.quizSubmissionForViewer(c, quiz, submission)},
	})
}

// decodeAnswerValue turns the `answer` JSON value into the string stored on
// quiz_submission_answers.answer. Strings are stored verbatim; arrays and
// objects (multiple_answer, ordering, matching, multiple_dropdown,
// categorization, hot_spot, file_upload …) are stored as their compact
// JSON text, which is exactly what the graders unmarshal; numbers and
// booleans keep their literal text; null / absent is "".
func decodeAnswerValue(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// maxQuizAnswerBytes bounds a single stored answer (essays included).
const maxQuizAnswerBytes = 256 * 1024

// AnswerQuestion handles PUT /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id
func (h *QuizSubmissionHandler) AnswerQuestion(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if _, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID)); wrote {
		return nil
	}

	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}

	submission, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}
	// Authorization: only the submission owner can answer questions.
	// 13.1.E: existence leak — return 404 not 403.
	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 || submission.UserID != userID {
		return responses.NotFound(c, "quiz submission")
	}

	// B024: the answer may be a string (MC id, typed text) or structured
	// JSON (arrays / objects for the newer item types).
	var input struct {
		Answer json.RawMessage `json:"answer"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	answerText, err := decodeAnswerValue(input.Answer)
	if err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if len(answerText) > maxQuizAnswerBytes {
		return responses.BadRequest(c, "Answer is too large")
	}

	answer, err := h.quizService.AnswerQuestion(c.Context(), submission.ID, uint(questionID), answerText)
	if err != nil {
		if errors.Is(err, service.ErrQuestionNotInSubmission) {
			return responses.NotFound(c, "quiz question")
		}
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(quizSubmissionAnswerToJSON(answer))
}

// CompleteSubmission handles POST /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/complete
func (h *QuizSubmissionHandler) CompleteSubmission(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	quiz, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID))
	if wrote {
		return nil
	}

	sub, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}

	userID, _ := c.Locals("user_id").(uint)
	if userID == 0 {
		return responses.Unauthorized(c)
	}

	submission, err := h.quizService.CompleteSubmission(c.Context(), sub.ID, userID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"quiz_submissions": []fiber.Map{h.quizSubmissionForViewer(c, quiz, submission)},
	})
}

// GetSubmissionAnswers handles GET /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/answers
//
// B033: the submission owner, course staff (SpeedGrader-style review, item
// analysis) and linked observers may read an attempt's answers; anyone
// else gets a 404.
func (h *QuizSubmissionHandler) GetSubmissionAnswers(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	quiz, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID))
	if wrote {
		return nil
	}

	sub, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}
	if !h.callerCanViewAttempt(c, sub.UserID) {
		return responses.NotFound(c, "quiz submission")
	}

	staff := quizCallerIsStaff(c)
	answers, err := h.quizService.ListSubmissionAnswers(c.Context(), sub.ID, staff)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Per-answer correctness, points and grader comments are the
	// attempt's results: a student (or observer) sees them only when the
	// quiz's result policy (hide_results, one-time results, post policy)
	// allows, as on the questions view.
	showResults := staff
	if !staff {
		now := time.Now()
		showResults = h.quizService.StudentResultsView(c.Context(), quiz, sub, now).Results
		if showResults && callerID(c) == sub.UserID {
			h.quizService.MarkResultsSeen(c.Context(), quiz, sub, now)
		}
	}
	result := make([]fiber.Map, len(answers))
	for i := range answers {
		result[i] = quizSubmissionAnswerToJSON(&answers[i])
		if !showResults {
			result[i]["correct"] = nil
			result[i]["points"] = nil
			result[i]["comment"] = ""
			result[i]["graded_via"] = nil
		}
	}

	// 13.5 PII audit — student-keyed read of a quiz attempt's answers.
	// Subject is the quiz submission's owner (sub.UserID).
	userID, _ := c.Locals("user_id").(uint)
	if userID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), userID, sub.UserID, "read", "quiz_answers", "quiz_submissions", sub.ID, c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"quiz_submission_answers": result,
	})
}

// callerID returns the authenticated user id (0 when absent).
func callerID(c *fiber.Ctx) uint {
	id, _ := c.Locals("user_id").(uint)
	return id
}

// GetSubmissionQuestions handles GET /courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions
// Returns the personalized set of questions for this submission (randomized
// from groups). Students only see the answer key once the attempt is over
// and the quiz's result policy allows it.
func (h *QuizSubmissionHandler) GetSubmissionQuestions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	quiz, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID))
	if wrote {
		return nil
	}

	submission, wrote := h.loadSubmission(c, uint(quizID))
	if wrote {
		return nil
	}

	// Authorization: only the submission owner, instructor, or observer can view
	// (13.1.E: 404, not 403).
	if !h.callerCanViewAttempt(c, submission.UserID) {
		return responses.NotFound(c, "quiz submission")
	}

	questions, err := h.quizService.GetSubmissionQuestions(c.Context(), submission.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch submission questions")
	}

	// G043: the answer key follows show_correct_answers, its show/hide
	// dates, "last attempt only", one-time results and hide_results.
	showKey := quizCallerIsStaff(c) || h.quizService.StudentResultsView(c.Context(), quiz, submission, time.Now()).AnswerKey
	stimuli := h.quizService.StimuliForQuestions(c.Context(), quiz.CourseID, questions)
	result := make([]fiber.Map, 0, len(questions))
	for i := range questions {
		q := &questions[i]
		if q.QuizID != submission.QuizID {
			continue // never serve a row from another quiz
		}
		result = append(result, quizQuestionToViewerJSON(q, showKey, service.QuizShuffleSeed(submission.ID, q.ID), stimuli))
	}

	return c.JSON(result)
}

// ListSubmissions handles GET /courses/:course_id/quizzes/:quiz_id/submissions.
//
// SECURITY (F-004): the quiz_id MUST belong to the URL's course_id within
// the caller's tenant (requireQuizInCourse). Course staff see every
// attempt; a student sees only their own attempts and an observer only
// their linked students' attempts (B034 — Canvas parity: students list
// their own quiz submissions).
func (h *QuizSubmissionHandler) ListSubmissions(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	quiz, wrote := h.requireQuizInCourse(c, uint(quizID), uint(courseID))
	if wrote {
		return nil
	}

	callerID, _ := c.Locals("user_id").(uint)

	if !quizCallerIsStaff(c) {
		if callerID == 0 {
			return responses.Unauthorized(c)
		}
		userIDs := []uint{callerID}
		if et, _ := c.Locals("enrollment_type").(string); et == "ObserverEnrollment" && h.observerService != nil {
			if observed, oErr := h.observerService.ListObservedStudents(c.Context(), callerID); oErr == nil {
				userIDs = append(userIDs, observed...)
			}
		}
		subs, err := h.quizService.ListSubmissionsForUsers(c.Context(), uint(quizID), userIDs)
		if err != nil {
			return responses.InternalError(c, "Could not fetch quiz submissions")
		}
		out := make([]fiber.Map, len(subs))
		for i := range subs {
			out[i] = h.quizSubmissionForViewer(c, quiz, &subs[i])
		}
		return c.JSON(fiber.Map{
			"quiz_submissions": out,
		})
	}

	params := middleware.GetPagination(c)

	result, err := h.quizService.ListSubmissions(c.Context(), uint(quizID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch quiz submissions")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	submissions := make([]fiber.Map, len(result.Items))
	for i := range result.Items {
		submissions[i] = quizSubmissionToJSON(&result.Items[i])
	}

	// 13.5 PII audit — bulk-read semantics. Per-row would explode the
	// audit table on large quiz rosters; one row per call with
	// student_id=0 + data_field="bulk_quiz_attempt_list_read" is what
	// audit investigations actually want (who pulled this quiz's
	// attempts, when).
	if callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_quiz_attempt_list_read", "quiz_submissions", uint(quizID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"quiz_submissions": submissions,
	})
}
