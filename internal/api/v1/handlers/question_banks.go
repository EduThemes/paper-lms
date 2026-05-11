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
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}

	bank, err := h.service.GetBank(c.Context(), uint(bankID))
	if err != nil {
		return responses.NotFound(c, "Question bank not found")
	}

	return c.JSON(bank)
}

func (h *QuestionBankHandler) UpdateBank(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}

	var input struct {
		Title string `json:"title"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	bank, err := h.service.UpdateBank(c.Context(), uint(bankID), input.Title)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(bank)
}

func (h *QuestionBankHandler) DeleteBank(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}

	if err := h.service.DeleteBank(c.Context(), uint(bankID)); err != nil {
		return responses.InternalError(c, "Could not delete question bank")
	}

	return c.JSON(fiber.Map{"deleted": true})
}

func (h *QuestionBankHandler) ListQuestions(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}

	questions, err := h.service.ListQuestions(c.Context(), uint(bankID))
	if err != nil {
		return responses.InternalError(c, "Could not list questions")
	}

	return c.JSON(questions)
}

func (h *QuestionBankHandler) AddQuestion(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}

	var entry models.QuestionBankEntry
	if err := c.BodyParser(&entry); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	entry.QuestionBankID = uint(bankID)

	if err := h.service.AddQuestion(c.Context(), &entry); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(201).JSON(entry)
}

func (h *QuestionBankHandler) UpdateQuestion(c *fiber.Ctx) error {
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}

	var entry models.QuestionBankEntry
	if err := c.BodyParser(&entry); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	updated, err := h.service.UpdateQuestion(c.Context(), uint(questionID), &entry)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(updated)
}

func (h *QuestionBankHandler) DeleteQuestion(c *fiber.Ctx) error {
	questionID, err := c.ParamsInt("question_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid question ID")
	}

	if err := h.service.DeleteQuestion(c.Context(), uint(questionID)); err != nil {
		return responses.InternalError(c, "Could not delete question")
	}

	return c.JSON(fiber.Map{"deleted": true})
}

func (h *QuestionBankHandler) PullToQuiz(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}

	var input struct {
		QuizID      uint   `json:"quiz_id"`
		QuestionIDs []uint `json:"question_ids"`
	}
	if err := c.BodyParser(&input); err != nil || input.QuizID == 0 {
		return responses.BadRequest(c, "quiz_id is required")
	}

	count, err := h.service.PullQuestionsToQuiz(c.Context(), uint(bankID), input.QuizID, input.QuestionIDs)
	if err != nil {
		return responses.InternalError(c, "Could not pull questions to quiz")
	}

	return c.JSON(fiber.Map{"questions_added": count})
}
