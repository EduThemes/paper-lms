package v1

import "github.com/gofiber/fiber/v2"

// registerQuizExtensionRoutes mounts the Wave A2 quizzing extensions
// (item banks, stimulus passages, per-question outcome alignment) and
// the Wave B QTI / IMSCC import + export endpoints. Both blocks layer
// on top of the core quiz CRUD registered in the main protected
// section; everything here can be added/removed without disturbing the
// underlying quiz contract.
func (r *Router) registerQuizExtensionRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	// Quiz Item Banks (course-scoped reusable question library).
	protected.Get("/courses/:course_id/quiz_item_banks", enrolled, r.QuizItemBankHandler.ListBanks)
	protected.Post("/courses/:course_id/quiz_item_banks", instructor, r.QuizItemBankHandler.CreateBank)
	protected.Get("/courses/:course_id/quiz_item_banks/:bank_id", enrolled, r.QuizItemBankHandler.GetBank)
	protected.Put("/courses/:course_id/quiz_item_banks/:bank_id", instructor, r.QuizItemBankHandler.UpdateBank)
	protected.Delete("/courses/:course_id/quiz_item_banks/:bank_id", instructor, r.QuizItemBankHandler.DeleteBank)

	// Quiz Item Bank Items (the reusable templates inside a bank).
	protected.Get("/quiz_item_banks/:bank_id/items", r.QuizItemBankHandler.ListBankItems)
	protected.Post("/quiz_item_banks/:bank_id/items", r.QuizItemBankHandler.CreateBankItem)
	protected.Get("/quiz_item_banks/:bank_id/items/:item_id", r.QuizItemBankHandler.GetBankItem)
	protected.Put("/quiz_item_banks/:bank_id/items/:item_id", r.QuizItemBankHandler.UpdateBankItem)
	protected.Delete("/quiz_item_banks/:bank_id/items/:item_id", r.QuizItemBankHandler.DeleteBankItem)

	// Quiz integration: copy an item into a quiz, or draw N random items from a bank.
	protected.Post("/quiz_item_banks/:bank_id/items/:item_id/add_to_quiz/:quiz_id", instructor, r.QuizItemBankHandler.AddBankItemToQuiz)
	protected.Post("/quiz_item_banks/:bank_id/random_draw", instructor, r.QuizItemBankHandler.RandomDraw)

	// Stimulus passages (TipTap docs shared across multiple quiz questions).
	protected.Get("/courses/:course_id/quiz_stimuli", enrolled, r.QuizStimulusHandler.ListStimuli)
	protected.Post("/courses/:course_id/quiz_stimuli", instructor, r.QuizStimulusHandler.CreateStimulus)
	protected.Get("/courses/:course_id/quiz_stimuli/:stimulus_id", enrolled, r.QuizStimulusHandler.GetStimulus)
	protected.Put("/courses/:course_id/quiz_stimuli/:stimulus_id", instructor, r.QuizStimulusHandler.UpdateStimulus)
	protected.Delete("/courses/:course_id/quiz_stimuli/:stimulus_id", instructor, r.QuizStimulusHandler.DeleteStimulus)
	protected.Get("/quiz_stimuli/:stimulus_id/questions", r.QuizStimulusHandler.ListQuestions)
	protected.Post("/quiz_stimuli/:stimulus_id/questions/:question_id", instructor, r.QuizStimulusHandler.LinkQuestion)
	protected.Delete("/quiz_stimuli/:stimulus_id/questions/:question_id", instructor, r.QuizStimulusHandler.UnlinkQuestion)

	// Per-question outcome alignment (data layer only — grader does not consume yet).
	protected.Get("/quiz_questions/:question_id/outcome_alignments", r.QuizOutcomeAlignmentHandler.ListByQuestion)
	protected.Post("/quiz_questions/:question_id/outcome_alignments", instructor, r.QuizOutcomeAlignmentHandler.Align)
	protected.Delete("/quiz_questions/:question_id/outcome_alignments/:outcome_id", instructor, r.QuizOutcomeAlignmentHandler.Unalign)
	protected.Get("/learning_outcomes/:outcome_id/quiz_question_alignments", r.QuizOutcomeAlignmentHandler.ListByOutcome)

	// Wave B: Canvas QTI / IMSCC import + export.
	// Sync-only in v1. The handler blocks while parsing + persisting;
	// Canvas-sized exports complete in well under a second.
	if r.QTIImportHandler != nil {
		protected.Post("/courses/:course_id/qti_import", instructor, r.QTIImportHandler.Import)
		protected.Get("/quizzes/:quiz_id/export.imscc", instructor, r.QTIImportHandler.Export)
	}
}
