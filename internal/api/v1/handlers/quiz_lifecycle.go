package handlers

// Quiz lifecycle endpoints (quiz-engine lane):
//
//   G042  PUT  .../quizzes/:quiz_id/submissions/:submission_id/scores     manual grading
//   G057  GET  .../quizzes/:quiz_id/submissions/:submission_id/events     attempt log (staff)
//   G057  POST .../quizzes/:quiz_id/submissions/:submission_id/events     take-page events (owner)
//   G047  GET  .../quizzes/:quiz_id/moderation                            moderate roster
//   G047  POST .../quizzes/:quiz_id/extensions                            extra attempts / time / unlock
//   G047  GET  .../quizzes/:quiz_id/attempt_status                        caller's attempts + dates
//   G053  POST .../quizzes/:quiz_id/questions/:question_id/regrade        regrade a question
//   G053  GET  .../quizzes/:quiz_id/regrades                              regrade history
//   G041  GET  .../assignments/:assignment_id/quiz                        quiz behind an online_quiz assignment
//
// Every route is course-scoped: the quiz is loaded under the caller's
// tenant and tied to :course_id (cross-tenant → 404), submissions are tied
// to :quiz_id and questions to :quiz_id.

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizLifecycleHandler struct {
	quizService *service.QuizService
}

func NewQuizLifecycleHandler(quizService *service.QuizService) *QuizLifecycleHandler {
	return &QuizLifecycleHandler{quizService: quizService}
}

// quizAndCourse parses :course_id / :quiz_id and loads the quiz for the
// caller. wrote=true means a response was written.
func (h *QuizLifecycleHandler) quizAndCourse(c *fiber.Ctx) (*models.Quiz, bool) {
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
	return loadQuizForCaller(c, h.quizService, uint(quizID), uint(courseID))
}

// attemptInQuiz loads :submission_id and ties it to the quiz.
func (h *QuizLifecycleHandler) attemptInQuiz(c *fiber.Ctx, quiz *models.Quiz) (*models.QuizSubmission, bool) {
	sid, err := c.ParamsInt("submission_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid submission ID")
		return nil, true
	}
	sub, err := h.quizService.GetSubmission(c.Context(), uint(sid))
	// G107: a section-limited grader reaches only their sections'
	// attempts (the course gate checks user-id params, not attempt ids).
	if err != nil || sub == nil || sub.QuizID != quiz.ID || !sectionVisibleUser(c, sub.UserID) {
		_ = responses.NotFound(c, "quiz submission")
		return nil, true
	}
	return sub, false
}

// attemptForStaffView is attemptInQuiz for the staff routes that expose
// an attempt's individual responses (manual scoring, the event log): on
// an anonymous survey those stay with the respondent (the same gate as
// the answers and file routes), so staff get a 404.
func (h *QuizLifecycleHandler) attemptForStaffView(c *fiber.Ctx, quiz *models.Quiz) (*models.QuizSubmission, bool) {
	sub, wrote := h.attemptInQuiz(c, quiz)
	if wrote {
		return nil, true
	}
	if anonymousSurveyHidden(c, quiz, sub) {
		_ = responses.NotFound(c, "quiz submission")
		return nil, true
	}
	return sub, false
}

// ---------- G042: manual grading ----------

// UpdateScores handles PUT .../submissions/:submission_id/scores.
//
// Body: {"fudge_points": 1.5, "questions": {"12": {"score": 3, "comment": "Good"}}}
// A question's score or comment may be omitted to leave it unchanged.
func (h *QuizLifecycleHandler) UpdateScores(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	sub, wrote := h.attemptForStaffView(c, quiz)
	if wrote {
		return nil
	}

	var input struct {
		FudgePoints optionalField[float64] `json:"fudge_points"`
		Questions   map[string]struct {
			Score   *float64 `json:"score"`
			Comment *string  `json:"comment"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	upd := service.QuizScoreUpdate{Questions: map[uint]service.QuestionScoreUpdate{}}
	if input.FudgePoints.Set {
		fp := 0.0
		if !input.FudgePoints.Null {
			fp = input.FudgePoints.Value
		}
		upd.FudgePoints = &fp
	}
	for key, q := range input.Questions {
		id, err := strconv.ParseUint(key, 10, 64)
		if err != nil || id == 0 {
			return responses.BadRequest(c, "Invalid question id")
		}
		upd.Questions[uint(id)] = service.QuestionScoreUpdate{Score: q.Score, Comment: q.Comment}
	}

	updated, answers, err := h.quizService.UpdateQuizScores(c.Context(), quiz, sub, upd, callerID(c))
	switch {
	case errors.Is(err, service.ErrQuestionNotInSubmission):
		return responses.NotFound(c, "quiz question")
	case errors.Is(err, service.ErrQuizAttemptNotFinished), errors.Is(err, service.ErrInvalidQuizScore),
		errors.Is(err, service.ErrGradingPeriodClosed):
		return responses.BadRequest(c, err.Error())
	case err != nil:
		return responses.InternalError(c, "Could not update quiz scores")
	}
	out := make([]fiber.Map, len(answers))
	for i := range answers {
		out[i] = quizSubmissionAnswerToJSON(&answers[i])
	}
	return c.JSON(fiber.Map{
		"quiz_submissions":        []fiber.Map{quizSubmissionToJSON(updated)},
		"quiz_submission_answers": out,
	})
}

// ---------- G057: event log ----------

func quizEventToJSON(e *models.QuizSubmissionEvent) fiber.Map {
	var data interface{}
	if e.EventData != "" {
		var obj map[string]interface{}
		if json.Unmarshal([]byte(e.EventData), &obj) == nil {
			data = obj
		}
	}
	return fiber.Map{
		"id":                 e.ID,
		"quiz_submission_id": e.QuizSubmissionID,
		"attempt":            e.Attempt,
		"event_type":         e.EventType,
		"event_data":         data,
		"source":             e.Source,
		"client_timestamp":   e.ClientTimestamp,
		"created_at":         e.CreatedAt,
	}
}

// ListEvents handles GET .../submissions/:submission_id/events (staff).
func (h *QuizLifecycleHandler) ListEvents(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	sub, wrote := h.attemptForStaffView(c, quiz)
	if wrote {
		return nil
	}
	events, err := h.quizService.ListQuizEvents(c.Context(), sub.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch quiz submission events")
	}
	out := make([]fiber.Map, len(events))
	for i := range events {
		out[i] = quizEventToJSON(&events[i])
	}
	return c.JSON(fiber.Map{"quiz_submission_events": out})
}

// RecordEvents handles POST .../submissions/:submission_id/events: the
// take page reports focus / blur / view / flag events for the caller's own
// in-progress attempt.
func (h *QuizLifecycleHandler) RecordEvents(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	sub, wrote := h.attemptInQuiz(c, quiz)
	if wrote {
		return nil
	}
	// Only the attempt's owner may write to its log (404, not 403).
	if uid := callerID(c); uid == 0 || uid != sub.UserID {
		return responses.NotFound(c, "quiz submission")
	}
	var input struct {
		Events []struct {
			EventType       string          `json:"event_type"`
			EventData       json.RawMessage `json:"event_data"`
			ClientTimestamp *time.Time      `json:"client_timestamp"`
		} `json:"quiz_submission_events"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	events := make([]service.ClientQuizEvent, len(input.Events))
	for i, e := range input.Events {
		events[i] = service.ClientQuizEvent{EventType: e.EventType, EventData: e.EventData, ClientTimestamp: e.ClientTimestamp}
	}
	n, err := h.quizService.RecordClientQuizEvents(c.Context(), sub, events)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"recorded": n})
}

// ---------- G047: moderate quiz ----------

// Moderation handles GET .../quizzes/:quiz_id/moderation (staff).
func (h *QuizLifecycleHandler) Moderation(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	rows, err := h.quizService.QuizModerationRoster(c.Context(), quiz, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not load the quiz roster")
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, r := range rows {
		if !sectionVisibleUser(c, r.UserID) { // G107
			continue
		}
		out = append(out, fiber.Map{
			"user_id":            r.UserID,
			"attempts_used":      r.AttemptsUsed,
			"attempts_left":      r.AttemptsLeft,
			"kept_score":         r.KeptScore,
			"latest_state":       r.LatestState,
			"in_progress_id":     r.InProgressID,
			"in_progress_end_at": r.InProgressEndAt,
			"extra_attempts":     r.ExtraAttempts,
			"extra_time":         r.ExtraTime,
			"manually_unlocked":  r.ManuallyUnlocked,
		})
	}
	return c.JSON(fiber.Map{
		"quiz":     quizToJSON(quiz),
		"students": out,
	})
}

// SetExtensions handles POST .../quizzes/:quiz_id/extensions (staff).
//
// Body: {"quiz_extensions": [{"user_id": 5, "extra_attempts": 1,
// "extra_time": 10, "manually_unlocked": true, "extend_from_now": 15}]}
func (h *QuizLifecycleHandler) SetExtensions(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	var input struct {
		Extensions []struct {
			UserID           uint  `json:"user_id"`
			ExtraAttempts    *int  `json:"extra_attempts"`
			ExtraTime        *int  `json:"extra_time"`
			ManuallyUnlocked *bool `json:"manually_unlocked"`
			ExtendFromNow    *int  `json:"extend_from_now"`
			ExtendFromEndAt  *int  `json:"extend_from_end_at"`
		} `json:"quiz_extensions"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil || len(input.Extensions) == 0 {
		return responses.BadRequest(c, "Invalid input")
	}
	if len(input.Extensions) > 500 {
		return responses.BadRequest(c, "Too many extensions in one request")
	}
	// Validate every row (target and values) first so a bad row writes
	// nothing.
	rows := make([]service.QuizExtensionInput, len(input.Extensions))
	for i, e := range input.Extensions {
		rows[i] = service.QuizExtensionInput{
			UserID:           e.UserID,
			ExtraAttempts:    e.ExtraAttempts,
			ExtraTime:        e.ExtraTime,
			ManuallyUnlocked: e.ManuallyUnlocked,
			ExtendFromNow:    e.ExtendFromNow,
			ExtendFromEndAt:  e.ExtendFromEndAt,
		}
		if e.UserID == 0 || !h.quizService.IsQuizCourseStudent(c.Context(), quiz.CourseID, e.UserID, callerAccountID(c)) || !sectionVisibleUser(c, e.UserID) { // G107
			return responses.NotFound(c, "student")
		}
		if err := rows[i].Validate(); err != nil {
			return responses.BadRequest(c, err.Error())
		}
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		ext, attempt, err := h.quizService.SetQuizExtension(c.Context(), quiz, row, callerID(c))
		if errors.Is(err, service.ErrInvalidQuizExtension) {
			return responses.BadRequest(c, err.Error())
		}
		if err != nil {
			return responses.InternalError(c, "Could not save the quiz extension")
		}
		saved := fiber.Map{
			"quiz_id":           ext.QuizID,
			"user_id":           ext.UserID,
			"extra_attempts":    ext.ExtraAttempts,
			"extra_time":        ext.ExtraTime,
			"manually_unlocked": ext.ManuallyUnlocked,
		}
		if attempt != nil {
			saved["end_at"] = attempt.EndAt
		}
		out = append(out, saved)
	}
	return c.JSON(fiber.Map{"quiz_extensions": out})
}

// AttemptStatus handles GET .../quizzes/:quiz_id/attempt_status: the
// caller's attempts used / left (extra attempts included), extra time and
// their own effective dates (overrides applied).
func (h *QuizLifecycleHandler) AttemptStatus(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	uid := callerID(c)
	if uid == 0 {
		return responses.Unauthorized(c)
	}
	st, err := h.quizService.AttemptStatusFor(c.Context(), quiz, uid, false)
	if err != nil {
		return responses.InternalError(c, "Could not load attempt status")
	}
	return c.JSON(fiber.Map{
		"attempts_used":     st.AttemptsUsed,
		"allowed_attempts":  st.AllowedAttempts,
		"attempts_left":     st.AttemptsLeft,
		"extra_time":        st.ExtraTime,
		"manually_unlocked": st.ManuallyUnlocked,
		"due_at":            st.DueAt,
		"unlock_at":         st.UnlockAt,
		"lock_at":           st.LockAt,
	})
}

// ---------- G053: regrade ----------

func quizRegradeToJSON(r *models.QuizQuestionRegrade) fiber.Map {
	return fiber.Map{
		"id":                   r.ID,
		"quiz_id":              r.QuizID,
		"quiz_question_id":     r.QuizQuestionID,
		"regrade_option":       r.RegradeOption,
		"answers_snapshot":     r.AnswersSnapshot,
		"regraded_by_id":       r.RegradedByID,
		"submissions_regraded": r.SubmissionsRegraded,
		"submissions_changed":  r.SubmissionsChanged,
		"created_at":           r.CreatedAt,
	}
}

// RegradeQuestion handles POST .../questions/:question_id/regrade (staff).
// Body: {"regrade_option": "current_correct_only"}.
func (h *QuizLifecycleHandler) RegradeQuestion(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	qid, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}
	question, err := h.quizService.GetQuestion(c.Context(), uint(qid))
	if err != nil || question == nil || question.QuizID != quiz.ID {
		return responses.NotFound(c, "quiz question")
	}
	var input struct {
		RegradeOption string `json:"regrade_option"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	rec, err := h.quizService.RegradeQuizQuestion(c.Context(), quiz, question, input.RegradeOption, callerID(c))
	if errors.Is(err, service.ErrInvalidRegradeOption) {
		return responses.BadRequest(c, err.Error())
	}
	if err != nil {
		return responses.InternalError(c, "Could not regrade the question")
	}
	return c.JSON(quizRegradeToJSON(rec))
}

// ListRegrades handles GET .../quizzes/:quiz_id/regrades (staff).
func (h *QuizLifecycleHandler) ListRegrades(c *fiber.Ctx) error {
	quiz, wrote := h.quizAndCourse(c)
	if wrote {
		return nil
	}
	list, err := h.quizService.ListQuizRegrades(c.Context(), quiz.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch regrades")
	}
	out := make([]fiber.Map, len(list))
	for i := range list {
		out[i] = quizRegradeToJSON(&list[i])
	}
	return c.JSON(out)
}

// ---------- G041: assignment → quiz ----------

// QuizForAssignment handles GET /courses/:course_id/assignments/:assignment_id/quiz:
// the quiz behind an online_quiz assignment, so the assignment page can
// send students to the quiz. Students only see a published quiz.
func (h *QuizLifecycleHandler) QuizForAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	aid, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}
	quiz, err := h.quizService.QuizForAssignment(c.Context(), uint(courseID), uint(aid))
	if err != nil || quiz == nil {
		return responses.NotFound(c, "quiz")
	}
	// Re-load under the caller's tenant + course + visibility rules.
	scoped, wrote := loadQuizForCaller(c, h.quizService, quiz.ID, uint(courseID))
	if wrote {
		return nil
	}
	return c.JSON(quizToJSON(scoped))
}
