package handlers

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type QuizQuestionHandler struct {
	quizService *service.QuizService
}

func NewQuizQuestionHandler(quizService *service.QuizService) *QuizQuestionHandler {
	return &QuizQuestionHandler{quizService: quizService}
}

// quizQuestionToJSON serializes a question WITH its answer key. Only use it
// for course staff; students get quizQuestionToViewerJSON.
func quizQuestionToJSON(q *models.QuizQuestion) fiber.Map {
	return fiber.Map{
		"id":                     q.ID,
		"quiz_id":                q.QuizID,
		"quiz_question_group_id": q.QuizQuestionGroupID,
		"position":               q.Position,
		"question_type":          service.NormalizeQuizQuestionType(q.QuestionType),
		"question_text":          q.QuestionText,
		"points_possible":        q.PointsPossible,
		"answers":                q.Answers,
		"correct_comments":       q.CorrectComments,
		"incorrect_comments":     q.IncorrectComments,
		"neutral_comments":       q.NeutralComments,
		"workflow_state":         q.WorkflowState,
		"bank_item_id":           q.BankItemID,
		"stimulus_id":            q.StimulusID,
		"question_bank_entry_id": q.QuestionBankEntryID,
		"created_at":             q.CreatedAt,
		"updated_at":             q.UpdatedAt,
	}
}

// quizQuestionToViewerJSON serializes a question for the caller: staff get
// the full answer key; everyone else gets a redacted copy (B023). The
// linked stimulus passage, if any, is embedded so the take page can render
// it (B126).
func quizQuestionToViewerJSON(q *models.QuizQuestion, showKey bool, seed int64, stimuli map[uint]*models.QuizStimulus) fiber.Map {
	m := quizQuestionToJSON(q)
	if !showKey {
		m = redactQuizQuestionJSON(m, q, seed)
	}
	if q.StimulusID != nil {
		if stim := stimuli[*q.StimulusID]; stim != nil {
			m["stimulus"] = quizStimulusToJSON(stim)
		}
	}
	return m
}

// scopedQuizQuestion loads the :question_id question and verifies it
// belongs to :quiz_id, which must itself live in :course_id and the
// caller's tenant. Returns wrote=true (a 404 has been written) on any
// miss so the caller short-circuits with `return nil`. Closes the
// nested-route cross-tenant / cross-quiz IDOR — the route middleware
// only guards :course_id.
func (h *QuizQuestionHandler) scopedQuizQuestion(c *fiber.Ctx) (*models.Quiz, *models.QuizQuestion, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, nil, true
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid quiz ID")
		return nil, nil, true
	}
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid question ID")
		return nil, nil, true
	}
	quiz, wrote := loadQuizForCaller(c, h.quizService, uint(quizID), uint(courseID))
	if wrote {
		return nil, nil, true
	}
	question, err := h.quizService.GetQuestion(c.Context(), uint(questionID))
	if err != nil || question.QuizID != uint(quizID) {
		_ = responses.NotFound(c, "quiz question")
		return nil, nil, true
	}
	return quiz, question, false
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
	quiz, wrote := loadQuizForCaller(c, h.quizService, uint(quizID), uint(courseID))
	if wrote {
		return nil
	}
	// A quiz gated by an access code / IP filter only shows its questions
	// inside an attempt (which enforces the gate).
	if !quizCallerIsStaff(c) && quizHasTakeRestrictions(quiz) {
		return c.JSON([]fiber.Map{})
	}

	params := middleware.GetPagination(c)

	result, err := h.quizService.ListQuestions(c.Context(), uint(quizID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch quiz questions")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	staff := quizCallerIsStaff(c)
	stimuli := h.quizService.StimuliForQuestions(c.Context(), quiz.CourseID, result.Items)
	questions := make([]fiber.Map, 0, len(result.Items))
	for i := range result.Items {
		q := &result.Items[i]
		// Per-attempt copies of bank entries are not part of the authored
		// quiz; students never see them outside an attempt.
		if !staff && q.QuestionBankEntryID != nil {
			continue
		}
		questions = append(questions, quizQuestionToViewerJSON(q, staff, service.QuizShuffleSeed(q.ID), stimuli))
	}

	return c.JSON(questions)
}

func (h *QuizQuestionHandler) GetQuestion(c *fiber.Ctx) error {
	quiz, question, wrote := h.scopedQuizQuestion(c)
	if wrote {
		return nil
	}
	if !quizCallerIsStaff(c) && quizHasTakeRestrictions(quiz) {
		return responses.NotFound(c, "quiz question")
	}

	stimuli := h.quizService.StimuliForQuestions(c.Context(), quiz.CourseID, []models.QuizQuestion{*question})
	return c.JSON(quizQuestionToViewerJSON(question, quizCallerIsStaff(c), service.QuizShuffleSeed(question.ID), stimuli))
}

// validateQuestionLinks checks that a group id belongs to the quiz and a
// stimulus id to the course. Returns wrote=true after writing a 400.
func (h *QuizQuestionHandler) validateQuestionLinks(c *fiber.Ctx, quiz *models.Quiz, groupID, stimulusID *uint) bool {
	if groupID != nil && *groupID != 0 {
		group, err := h.quizService.GetQuestionGroup(c.Context(), *groupID)
		if err != nil || group == nil || group.QuizID != quiz.ID {
			_ = responses.BadRequest(c, "quiz_question_group_id does not belong to this quiz")
			return true
		}
	}
	if stimulusID != nil && *stimulusID != 0 {
		if err := h.quizService.ValidateStimulusForCourse(c.Context(), *stimulusID, quiz.CourseID); err != nil {
			_ = responses.BadRequest(c, "stimulus_id does not belong to this course")
			return true
		}
	}
	return false
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
	quiz, wrote := loadQuizForCaller(c, h.quizService, uint(quizID), uint(courseID))
	if wrote {
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
			StimulusID          *uint    `json:"stimulus_id"`
		} `json:"question"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if h.validateQuestionLinks(c, quiz, input.Question.QuizQuestionGroupID, input.Question.StimulusID) {
		return nil
	}

	question := &models.QuizQuestion{
		QuizID:              uint(quizID),
		QuizQuestionGroupID: input.Question.QuizQuestionGroupID,
		Position:            input.Question.Position,
		QuestionType:        input.Question.QuestionType,
		QuestionText:        service.SanitizeHTML(input.Question.QuestionText),
		PointsPossible:      input.Question.PointsPossible,
		Answers:             input.Question.Answers,
		CorrectComments:     service.SanitizeHTML(input.Question.CorrectComments),
		IncorrectComments:   service.SanitizeHTML(input.Question.IncorrectComments),
		NeutralComments:     service.SanitizeHTML(input.Question.NeutralComments),
	}
	if input.Question.StimulusID != nil && *input.Question.StimulusID != 0 {
		question.StimulusID = input.Question.StimulusID
	}
	if question.QuizQuestionGroupID != nil && *question.QuizQuestionGroupID == 0 {
		question.QuizQuestionGroupID = nil
	}

	if err := h.quizService.CreateQuestion(c.Context(), question); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(quizQuestionToJSON(question))
}

func (h *QuizQuestionHandler) UpdateQuestion(c *fiber.Ctx) error {
	quiz, question, wrote := h.scopedQuizQuestion(c)
	if wrote {
		return nil
	}

	// quiz_question_group_id and stimulus_id are optionalField so an
	// explicit null clears the link (B129 / B126); an absent key leaves it.
	var input struct {
		Question struct {
			Position            *int                `json:"position"`
			QuestionType        *string             `json:"question_type"`
			QuestionText        *string             `json:"question_text"`
			PointsPossible      *float64            `json:"points_possible"`
			Answers             *string             `json:"answers"`
			CorrectComments     *string             `json:"correct_comments"`
			IncorrectComments   *string             `json:"incorrect_comments"`
			NeutralComments     *string             `json:"neutral_comments"`
			QuizQuestionGroupID optionalField[uint] `json:"quiz_question_group_id"`
			StimulusID          optionalField[uint] `json:"stimulus_id"`
		} `json:"question"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	var newGroup, newStimulus *uint
	if input.Question.QuizQuestionGroupID.Set && !input.Question.QuizQuestionGroupID.Null {
		v := input.Question.QuizQuestionGroupID.Value
		newGroup = &v
	}
	if input.Question.StimulusID.Set && !input.Question.StimulusID.Null {
		v := input.Question.StimulusID.Value
		newStimulus = &v
	}
	if h.validateQuestionLinks(c, quiz, newGroup, newStimulus) {
		return nil
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
		question.CorrectComments = service.SanitizeHTML(*input.Question.CorrectComments)
	}
	if input.Question.IncorrectComments != nil {
		question.IncorrectComments = service.SanitizeHTML(*input.Question.IncorrectComments)
	}
	if input.Question.NeutralComments != nil {
		question.NeutralComments = service.SanitizeHTML(*input.Question.NeutralComments)
	}
	if input.Question.QuizQuestionGroupID.Set {
		if newGroup == nil || *newGroup == 0 {
			question.QuizQuestionGroupID = nil
		} else {
			question.QuizQuestionGroupID = newGroup
		}
	}
	if input.Question.StimulusID.Set {
		if newStimulus == nil || *newStimulus == 0 {
			question.StimulusID = nil
		} else {
			question.StimulusID = newStimulus
		}
	}

	if err := h.quizService.UpdateQuestion(c.Context(), question); err != nil {
		return responses.InternalError(c, "Could not update quiz question")
	}

	return c.JSON(quizQuestionToJSON(question))
}

func (h *QuizQuestionHandler) DeleteQuestion(c *fiber.Ctx) error {
	_, question, wrote := h.scopedQuizQuestion(c)
	if wrote {
		return nil
	}

	if err := h.quizService.DeleteQuestion(c.Context(), question.ID); err != nil {
		return responses.InternalError(c, "Could not delete quiz question")
	}

	return c.JSON(fiber.Map{"delete": true})
}
