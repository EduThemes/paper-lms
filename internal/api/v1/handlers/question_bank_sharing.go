package handlers

// Question-bank sharing endpoints (quiz-items lane, G060): read access to
// banks shared with the course's account, copying a bank into the course,
// and a random draw into a quiz.

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// usableBank loads :bank_id for READ use from :course_id: a bank the course
// owns, or an active bank another course in the same account shared with
// the account. Anything else is a 404 (no existence leak). Returns the bank,
// the course id and wrote=true after writing an error.
func (h *QuestionBankHandler) usableBank(c *fiber.Ctx) (*models.QuestionBank, uint, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, 0, true
	}
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid bank ID")
		return nil, 0, true
	}
	bank, err := h.service.GetBank(c.Context(), uint(bankID))
	// Banks shared from other courses are a staff resource; students only
	// ever reach their own course's banks.
	if err == nil && bank != nil && bank.CourseID != uint(courseID) && !quizCallerIsStaff(c) {
		bank = nil
	}
	if err != nil || bank == nil || !h.service.BankUsableInCourse(c.Context(), bank, uint(courseID)) {
		_ = responses.NotFound(c, "question bank")
		return nil, 0, true
	}
	return bank, uint(courseID), false
}

// questionBankJSON adds whether the requesting course owns the bank.
func questionBankJSON(b *models.QuestionBank, courseID uint) fiber.Map {
	return fiber.Map{
		"id":                  b.ID,
		"course_id":           b.CourseID,
		"title":               b.Title,
		"description":         b.Description,
		"workflow_state":      b.WorkflowState,
		"shared_with_account": b.SharedWithAccount,
		"owned":               b.CourseID == courseID,
		"created_at":          b.CreatedAt,
		"updated_at":          b.UpdatedAt,
	}
}

// parseBankEntryInput decodes a bank question body. `answers` may be the
// JSON array itself or that array encoded as a string (the quiz editor's
// convention); it is stored as compact JSON text. question_text is rich
// content; the name and feedback are plain text.
func parseBankEntryInput(c *fiber.Ctx) (*models.QuestionBankEntry, bool) {
	var in struct {
		QuestionName      string          `json:"question_name"`
		QuestionType      string          `json:"question_type"`
		QuestionText      string          `json:"question_text"`
		PointsPossible    *float64        `json:"points_possible"`
		Answers           json.RawMessage `json:"answers"`
		Feedback          string          `json:"feedback"`
		Position          int             `json:"position"`
		CorrectComments   string          `json:"correct_comments"`
		IncorrectComments string          `json:"incorrect_comments"`
		NeutralComments   string          `json:"neutral_comments"`
	}
	if err := json.Unmarshal(c.Body(), &in); err != nil {
		_ = responses.BadRequest(c, "Invalid input")
		return nil, true
	}
	answers, err := decodeAnswerValue(in.Answers)
	if err != nil {
		_ = responses.BadRequest(c, "answers must be a JSON array")
		return nil, true
	}
	if strings.TrimSpace(answers) == "" {
		answers = "[]"
	}
	if !json.Valid([]byte(answers)) {
		_ = responses.BadRequest(c, "answers must be a JSON array")
		return nil, true
	}
	pts := 1.0
	if in.PointsPossible != nil {
		pts = *in.PointsPossible
	}
	return &models.QuestionBankEntry{
		QuestionName:      service.SanitizePlainText(in.QuestionName),
		QuestionType:      service.NormalizeQuizQuestionType(in.QuestionType),
		QuestionText:      service.SanitizeHTML(in.QuestionText),
		PointsPossible:    pts,
		Answers:           answers,
		Feedback:          service.SanitizePlainText(in.Feedback),
		Position:          in.Position,
		CorrectComments:   service.SanitizePlainText(in.CorrectComments),
		IncorrectComments: service.SanitizePlainText(in.IncorrectComments),
		NeutralComments:   service.SanitizePlainText(in.NeutralComments),
	}, false
}

// CopyBank handles POST /courses/:course_id/question_banks/:bank_id/copy:
// copies an own or shared bank (with its questions) into this course.
func (h *QuestionBankHandler) CopyBank(c *fiber.Ctx) error {
	bank, courseID, wrote := h.usableBank(c)
	if wrote {
		return nil
	}
	userID, _ := c.Locals("user_id").(uint)
	copied, n, err := h.service.CopyBank(c.Context(), bank, courseID, userID)
	if err != nil {
		return responses.InternalError(c, "Could not copy question bank")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"question_bank": questionBankJSON(copied, courseID), "questions_copied": n})
}

// RandomDraw handles POST /courses/:course_id/question_banks/:bank_id/random_draw
// with {"quiz_id": N, "count": K}: copies K random questions into the quiz.
func (h *QuestionBankHandler) RandomDraw(c *fiber.Ctx) error {
	bank, courseID, wrote := h.usableBank(c)
	if wrote {
		return nil
	}
	var in struct {
		QuizID uint `json:"quiz_id"`
		Count  int  `json:"count"`
	}
	if err := c.BodyParser(&in); err != nil || in.QuizID == 0 {
		return responses.BadRequest(c, "quiz_id is required")
	}
	if in.Count < 1 || in.Count > 500 {
		return responses.BadRequest(c, "count must be between 1 and 500")
	}
	n, err := h.service.DrawRandomToQuiz(c.Context(), bank, courseID, in.QuizID, callerAccountID(c), in.Count)
	if err != nil {
		if errors.Is(err, service.ErrBankNotUsable) {
			return responses.NotFound(c, "question bank")
		}
		if errors.Is(err, service.ErrTargetQuizNotInCourse) {
			return responses.NotFound(c, "quiz")
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"questions_added": n})
}
