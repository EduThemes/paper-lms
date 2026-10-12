package handlers

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// quizCallerIsStaff reports whether the caller may see quiz answer keys and
// every student's attempts in the course on the request path: a course
// Teacher/TA, or an admin. Mirrors the `instructor` route middleware
// (RequireInstructor = Teacher/TA, admins pass).
func quizCallerIsStaff(c *fiber.Ctx) bool {
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		return true
	}
	switch et, _ := c.Locals("enrollment_type").(string); et {
	case "TeacherEnrollment", "TaEnrollment":
		return true
	}
	return false
}

// quizVisibleToStudents reports whether a quiz is published and live, i.e.
// whether a non-staff caller may see or take it (Canvas: unpublished and
// deleted quizzes are invisible to students).
func quizVisibleToStudents(q *models.Quiz) bool {
	if q == nil || q.WorkflowState == "deleted" {
		return false
	}
	return q.Published || q.WorkflowState == "published"
}

// loadQuizForCaller loads :quiz_id scoped to the caller's tenant and the
// URL's course (F-003/F-004 tie, same as requireQuizInCourse) and, for
// non-staff callers, additionally requires the quiz to be published and not
// deleted (B128). Returns wrote=true after writing a 404; the caller must
// short-circuit with `return nil`.
func loadQuizForCaller(c *fiber.Ctx, qs *service.QuizService, quizID, courseID uint) (*models.Quiz, bool) {
	quiz, err := qs.GetQuizScoped(c.Context(), quizID, callerAccountID(c))
	if err != nil || quiz == nil || quiz.CourseID != courseID || quiz.WorkflowState == "deleted" {
		_ = responses.NotFound(c, "quiz")
		return nil, true
	}
	if !quizCallerIsStaff(c) && !quizVisibleToStudents(quiz) {
		_ = responses.NotFound(c, "quiz")
		return nil, true
	}
	return quiz, false
}

// redactQuizQuestionJSON strips every answer-key field from a serialized
// question: answers are reduced to what the item player needs to render,
// and per-question feedback (which frequently states the answer) is
// dropped. seed drives the stable shuffle of order-revealing lists.
func redactQuizQuestionJSON(m fiber.Map, q *models.QuizQuestion, seed int64) fiber.Map {
	m["answers"] = service.StudentSafeQuizAnswers(q.QuestionType, q.Answers, seed)
	m["correct_comments"] = ""
	m["incorrect_comments"] = ""
	m["neutral_comments"] = ""
	return m
}

// optionalField distinguishes "absent" from an explicit JSON null in a
// PATCH-style body: Set is true when the key was present; Null is true when
// its value was null. Value holds the decoded value otherwise.
type optionalField[T any] struct {
	Set   bool
	Null  bool
	Value T
}

func (o *optionalField[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}
