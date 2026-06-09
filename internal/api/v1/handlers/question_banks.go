package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuestionBankHandler struct {
	service *service.QuestionBankService
}

func NewQuestionBankHandler(service *service.QuestionBankService) *QuestionBankHandler {
	return &QuestionBankHandler{service: service}
}

// scopedBank loads :bank_id and verifies it lives in :course_id and the
// caller's tenant (the route middleware pins :course_id to an
// enrollment; bank.CourseID == :course_id transitively enforces tenant).
// Returns wrote=true (404 written) on any miss so the caller
// short-circuits with `return nil`. Closes the nested-route IDOR — every
// bank op was previously addressable by raw id across tenants.
func (h *QuestionBankHandler) scopedBank(c *fiber.Ctx) (*models.QuestionBank, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid bank ID")
		return nil, true
	}
	bank, err := h.service.GetBank(c.Context(), uint(bankID))
	if err != nil || bank == nil || bank.CourseID != uint(courseID) {
		_ = responses.NotFound(c, "question bank")
		return nil, true
	}
	return bank, false
}

// scopedBankEntry loads :question_id and verifies it belongs to
// :bank_id, which must itself live in :course_id and the caller's tenant.
// Returns wrote=true (404 written) on any miss.
func (h *QuestionBankHandler) scopedBankEntry(c *fiber.Ctx) (*models.QuestionBankEntry, bool) {
	bank, wrote := h.scopedBank(c)
	if wrote {
		return nil, true
	}
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid question ID")
		return nil, true
	}
	entry, err := h.service.GetEntry(c.Context(), uint(questionID))
	if err != nil || entry.QuestionBankID != bank.ID {
		_ = responses.NotFound(c, "question")
		return nil, true
	}
	return entry, false
}

func (h *QuestionBankHandler) ListBanks(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	banks, err := h.service.ListBanks(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not list question banks")
	}

	responses.SetPaginationHeaders(c, banks.TotalCount, params.Page, params.PerPage)
	return c.JSON(banks.Items)
}

func (h *QuestionBankHandler) CreateBank(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Title string `json:"title"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	bank := &models.QuestionBank{
		CourseID: uint(courseID),
		Title:    input.Title,
	}
	if err := h.service.CreateBank(c.Context(), bank); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(201).JSON(bank)
}

func (h *QuestionBankHandler) GetBank(c *fiber.Ctx) error {
	bank, wrote := h.scopedBank(c)
	if wrote {
		return nil
	}

	return c.JSON(bank)
}

func (h *QuestionBankHandler) UpdateBank(c *fiber.Ctx) error {
	bank, wrote := h.scopedBank(c)
	if wrote {
		return nil
	}

	var input struct {
		Title string `json:"title"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	updated, err := h.service.UpdateBank(c.Context(), bank.ID, input.Title)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(updated)
}

func (h *QuestionBankHandler) DeleteBank(c *fiber.Ctx) error {
	bank, wrote := h.scopedBank(c)
	if wrote {
		return nil
	}

	if err := h.service.DeleteBank(c.Context(), bank.ID); err != nil {
		return responses.InternalError(c, "Could not delete question bank")
	}

	return c.JSON(fiber.Map{"deleted": true})
}

func (h *QuestionBankHandler) ListQuestions(c *fiber.Ctx) error {
	bank, wrote := h.scopedBank(c)
	if wrote {
		return nil
	}

	questions, err := h.service.ListQuestions(c.Context(), bank.ID)
	if err != nil {
		return responses.InternalError(c, "Could not list questions")
	}

	return c.JSON(questions)
}

func (h *QuestionBankHandler) AddQuestion(c *fiber.Ctx) error {
	bank, wrote := h.scopedBank(c)
	if wrote {
		return nil
	}

	var entry models.QuestionBankEntry
	if err := c.BodyParser(&entry); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	// Server-set from the scoped path, never the body — prevents
	// inserting an entry into another bank/tenant.
	entry.QuestionBankID = bank.ID

	if err := h.service.AddQuestion(c.Context(), &entry); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(201).JSON(entry)
}

func (h *QuestionBankHandler) UpdateQuestion(c *fiber.Ctx) error {
	existing, wrote := h.scopedBankEntry(c)
	if wrote {
		return nil
	}

	var entry models.QuestionBankEntry
	if err := c.BodyParser(&entry); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	updated, err := h.service.UpdateQuestion(c.Context(), existing.ID, &entry)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(updated)
}

func (h *QuestionBankHandler) DeleteQuestion(c *fiber.Ctx) error {
	existing, wrote := h.scopedBankEntry(c)
	if wrote {
		return nil
	}

	if err := h.service.DeleteQuestion(c.Context(), existing.ID); err != nil {
		return responses.InternalError(c, "Could not delete question")
	}

	return c.JSON(fiber.Map{"deleted": true})
}

func (h *QuestionBankHandler) PullToQuiz(c *fiber.Ctx) error {
	bank, wrote := h.scopedBank(c)
	if wrote {
		return nil
	}

	var input struct {
		QuizID      uint   `json:"quiz_id"`
		QuestionIDs []uint `json:"question_ids"`
	}
	if err := c.BodyParser(&input); err != nil || input.QuizID == 0 {
		return responses.BadRequest(c, "quiz_id is required")
	}

	count, err := h.service.PullQuestionsToQuiz(c.Context(), bank.ID, input.QuizID, callerAccountID(c), input.QuestionIDs)
	if err != nil {
		return responses.InternalError(c, "Could not pull questions to quiz")
	}

	return c.JSON(fiber.Map{"questions_added": count})
}
