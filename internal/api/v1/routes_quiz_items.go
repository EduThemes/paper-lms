package v1

import "github.com/gofiber/fiber/v2"

// registerQuizItemsRoutes mounts the quiz-items lane endpoints: the
// can't-go-back question lock, the formula authoring preview, and the
// item-analysis report (JSON + CSV).
func (r *Router) registerQuizItemsRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	// Can't go back: the student moved past a question (owner only; the
	// handler ties submission -> quiz -> course -> tenant).
	protected.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id/read", enrolled, r.QuizSubmissionHandler.MarkQuestionRead)

	// Formula (calculated) question preview: sample variable sets + answers.
	protected.Post("/courses/:course_id/quizzes/:quiz_id/formula_preview", instructor, r.QuizQuestionHandler.FormulaPreview)

	// Question banks (single bank system): copy an own/shared bank into
	// the course, or draw N random questions into a quiz.
	protected.Post("/courses/:course_id/question_banks/:bank_id/copy", instructor, r.QuestionBankHandler.CopyBank)
	protected.Post("/courses/:course_id/question_banks/:bank_id/random_draw", instructor, r.QuestionBankHandler.RandomDraw)

	// Item analysis + CSV reports (instructor only).
	protected.Get("/courses/:course_id/quizzes/:quiz_id/item_analysis", instructor, r.QuizStatisticsHandler.GetItemAnalysis)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/item_analysis.csv", instructor, r.QuizStatisticsHandler.ItemAnalysisCSV)
	protected.Get("/courses/:course_id/quizzes/:quiz_id/student_analysis.csv", instructor, r.QuizStatisticsHandler.StudentAnalysisCSV)
}
