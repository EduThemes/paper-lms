package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type QuizItemBankHandler struct {
	svc *service.QuizItemBankService
}

func NewQuizItemBankHandler(svc *service.QuizItemBankService) *QuizItemBankHandler {
	return &QuizItemBankHandler{svc: svc}
}

func quizItemBankToJSON(b *models.QuizItemBank) fiber.Map {
	return fiber.Map{
		"id":                 b.ID,
		"course_id":          b.CourseID,
		"title":              b.Title,
		"description":        b.Description,
		"created_by_user_id": b.CreatedByUserID,
		"created_at":         b.CreatedAt,
		"updated_at":         b.UpdatedAt,
	}
}

func quizItemBankItemToJSON(i *models.QuizItemBankItem) fiber.Map {
	return fiber.Map{
		"id":                 i.ID,
		"bank_id":            i.BankID,
		"position":           i.Position,
		"question_type":      i.QuestionType,
		"question_text":      i.QuestionText,
		"points_possible":    i.PointsPossible,
		"answers":            i.Answers,
		"correct_comments":   i.CorrectComments,
		"incorrect_comments": i.IncorrectComments,
		"neutral_comments":   i.NeutralComments,
		"created_at":         i.CreatedAt,
		"updated_at":         i.UpdatedAt,
	}
}

// ---------- Banks ----------

func (h *QuizItemBankHandler) ListBanks(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	params := middleware.GetPagination(c)
	result, err := h.svc.ListBanks(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch item banks")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
	out := make([]fiber.Map, len(result.Items))
	for i, b := range result.Items {
		out[i] = quizItemBankToJSON(&b)
	}
	return c.JSON(out)
}

func (h *QuizItemBankHandler) GetBank(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}
	bank, err := h.svc.GetBank(c.Context(), uint(courseID), uint(id))
	if err != nil {
		return responses.NotFound(c, "item bank")
	}
	return c.JSON(quizItemBankToJSON(bank))
}

func (h *QuizItemBankHandler) CreateBank(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		Bank struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"bank"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	userID, _ := c.Locals("user_id").(uint)
	bank := &models.QuizItemBank{
		CourseID:        uint(courseID),
		Title:           input.Bank.Title,
		Description:     input.Bank.Description,
		CreatedByUserID: userID,
	}
	if err := h.svc.CreateBank(c.Context(), bank); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizItemBankToJSON(bank))
}

func (h *QuizItemBankHandler) UpdateBank(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}
	existing, err := h.svc.GetBank(c.Context(), uint(courseID), uint(id))
	if err != nil {
		return responses.NotFound(c, "item bank")
	}
	var input struct {
		Bank struct {
			Title       *string `json:"title"`
			Description *string `json:"description"`
		} `json:"bank"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Bank.Title != nil {
		existing.Title = *input.Bank.Title
	}
	if input.Bank.Description != nil {
		existing.Description = *input.Bank.Description
	}
	if err := h.svc.UpdateBank(c.Context(), uint(courseID), existing); err != nil {
		return responses.InternalError(c, "Could not update item bank")
	}
	return c.JSON(quizItemBankToJSON(existing))
}

func (h *QuizItemBankHandler) DeleteBank(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	id, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}
	if err := h.svc.DeleteBank(c.Context(), uint(courseID), uint(id)); err != nil {
		return responses.NotFound(c, "item bank")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// ---------- Bank Items ----------

func (h *QuizItemBankHandler) ListBankItems(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}
	items, err := h.svc.ListBankItems(c.Context(), uint(bankID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch bank items")
	}
	out := make([]fiber.Map, len(items))
	for i, it := range items {
		out[i] = quizItemBankItemToJSON(&it)
	}
	return c.JSON(out)
}

func (h *QuizItemBankHandler) CreateBankItem(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}
	var input struct {
		Item struct {
			Position          int      `json:"position"`
			QuestionType      string   `json:"question_type"`
			QuestionText      string   `json:"question_text"`
			PointsPossible    *float64 `json:"points_possible"`
			Answers           string   `json:"answers"`
			CorrectComments   string   `json:"correct_comments"`
			IncorrectComments string   `json:"incorrect_comments"`
			NeutralComments   string   `json:"neutral_comments"`
		} `json:"item"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	item := &models.QuizItemBankItem{
		BankID:            uint(bankID),
		Position:          input.Item.Position,
		QuestionType:      input.Item.QuestionType,
		QuestionText:      input.Item.QuestionText,
		PointsPossible:    input.Item.PointsPossible,
		Answers:           input.Item.Answers,
		CorrectComments:   input.Item.CorrectComments,
		IncorrectComments: input.Item.IncorrectComments,
		NeutralComments:   input.Item.NeutralComments,
	}
	if err := h.svc.CreateBankItem(c.Context(), item); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizItemBankItemToJSON(item))
}

func (h *QuizItemBankHandler) GetBankItem(c *fiber.Ctx) error {
	id, err := c.ParamsInt("item_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid item ID")
	}
	item, err := h.svc.GetBankItem(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "bank item")
	}
	return c.JSON(quizItemBankItemToJSON(item))
}

func (h *QuizItemBankHandler) UpdateBankItem(c *fiber.Ctx) error {
	id, err := c.ParamsInt("item_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid item ID")
	}
	item, err := h.svc.GetBankItem(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "bank item")
	}
	var input struct {
		Item struct {
			Position          *int     `json:"position"`
			QuestionType      *string  `json:"question_type"`
			QuestionText      *string  `json:"question_text"`
			PointsPossible    *float64 `json:"points_possible"`
			Answers           *string  `json:"answers"`
			CorrectComments   *string  `json:"correct_comments"`
			IncorrectComments *string  `json:"incorrect_comments"`
			NeutralComments   *string  `json:"neutral_comments"`
		} `json:"item"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.Item.Position != nil {
		item.Position = *input.Item.Position
	}
	if input.Item.QuestionType != nil {
		item.QuestionType = *input.Item.QuestionType
	}
	if input.Item.QuestionText != nil {
		item.QuestionText = *input.Item.QuestionText
	}
	if input.Item.PointsPossible != nil {
		item.PointsPossible = input.Item.PointsPossible
	}
	if input.Item.Answers != nil {
		item.Answers = *input.Item.Answers
	}
	if input.Item.CorrectComments != nil {
		item.CorrectComments = *input.Item.CorrectComments
	}
	if input.Item.IncorrectComments != nil {
		item.IncorrectComments = *input.Item.IncorrectComments
	}
	if input.Item.NeutralComments != nil {
		item.NeutralComments = *input.Item.NeutralComments
	}
	if err := h.svc.UpdateBankItem(c.Context(), item); err != nil {
		return responses.InternalError(c, "Could not update bank item")
	}
	return c.JSON(quizItemBankItemToJSON(item))
}

func (h *QuizItemBankHandler) DeleteBankItem(c *fiber.Ctx) error {
	id, err := c.ParamsInt("item_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid item ID")
	}
	if err := h.svc.DeleteBankItem(c.Context(), uint(id)); err != nil {
		return responses.NotFound(c, "bank item")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// ---------- Quiz integration ----------

// AddBankItemToQuiz copies a bank item into the given quiz as a QuizQuestion.
// POST /api/v1/quiz_item_banks/:bank_id/items/:item_id/add_to_quiz/:quiz_id
func (h *QuizItemBankHandler) AddBankItemToQuiz(c *fiber.Ctx) error {
	itemID, err := c.ParamsInt("item_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid item ID")
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	var input struct {
		Position int `json:"position"`
	}
	_ = c.BodyParser(&input)
	q, err := h.svc.AddBankItemToQuiz(c.Context(), uint(itemID), uint(quizID), input.Position)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizQuestionToJSON(q))
}

// RandomDraw returns N randomly-shuffled bank items shaped like QuizQuestions.
// POST /api/v1/quiz_item_banks/:bank_id/random_draw  body {"count": 5}
func (h *QuizItemBankHandler) RandomDraw(c *fiber.Ctx) error {
	bankID, err := c.ParamsInt("bank_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid bank ID")
	}
	var input struct {
		Count int `json:"count"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	questions, err := h.svc.RandomDrawFromBank(c.Context(), uint(bankID), input.Count)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	out := make([]fiber.Map, len(questions))
	for i, q := range questions {
		out[i] = quizQuestionToJSON(&q)
	}
	return c.JSON(out)
}
