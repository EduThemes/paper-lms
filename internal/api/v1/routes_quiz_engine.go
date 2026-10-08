package v1

import (
	"github.com/gofiber/fiber/v2"
)

// registerQuizEngineRoutes mounts the quiz lifecycle endpoints (quiz-engine
// lane): manual grading, attempt event log, moderation, regrade and the
// assignment → quiz lookup. Staff-only routes use `instructor`; the
// handler ties every :quiz_id to :course_id under the caller's tenant.
func (r *Router) registerQuizEngineRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	h := r.QuizLifecycleHandler
	if h == nil {
		return
	}
	// G042 manual grading (scores, comments, fudge points).
	protected.Put("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/scores", instructor, h.UpdateScores)
	// G057 attempt event log: staff read, the attempt owner writes.
	protected.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/events", instructor, h.ListEvents)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/events", enrolled, h.RecordEvents)
	// G047 moderate quiz.
	protected.Get("/courses/:course_id/quizzes/:quiz_id/moderation", instructor, h.Moderation)
	protected.Post("/courses/:course_id/quizzes/:quiz_id/extensions", instructor, h.SetExtensions)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/attempt_status", enrolled, h.AttemptStatus)
	// G053 regrade.
	protected.Post("/courses/:course_id/quizzes/:quiz_id/questions/:question_id/regrade", instructor, h.RegradeQuestion)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/regrades", instructor, h.ListRegrades)
	// G041 the quiz behind an online_quiz assignment.
	protected.Get("/courses/:course_id/assignments/:assignment_id/quiz", enrolled, h.QuizForAssignment)
}
