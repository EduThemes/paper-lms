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
		Description:     service.SanitizeHTML(input.Bank.Description),
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
		existing.Description = service.SanitizeHTML(*input.Bank.Description)
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
//
// SECURITY (B032): every item route is mounted under
// /courses/:course_id/quiz_item_banks/:bank_id/items behind the
// `instructor` middleware, and each handler ties the bank to :course_id
// and the item to :bank_id. Previously the routes were course-less and
// unguarded, so any authenticated user in any tenant could read answer
// keys or modify / delete items by raw id.

// scopedItemBank loads :bank_id and verifies it lives in :course_id.
// Returns wrote=true after writing an error response.
func (h *QuizItemBankHandler) scopedItemBank(c *fiber.Ctx) (*models.QuizItemBank, bool) {
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
	if courseID <= 0 {
		_ = responses.NotFound(c, "item bank")
		return nil, true
	}
	bank, err := h.svc.GetBank(c.Context(), uint(courseID), uint(bankID))
	if err != nil || bank == nil || bank.CourseID != uint(courseID) {
		_ = responses.NotFound(c, "item bank")
		return nil, true
	}
	return bank, false
}

// scopedBankItem loads :item_id and verifies it belongs to the scoped bank.
func (h *QuizItemBankHandler) scopedBankItem(c *fiber.Ctx) (*models.QuizItemBank, *models.QuizItemBankItem, bool) {
	bank, wrote := h.scopedItemBank(c)
	if wrote {
		return nil, nil, true
	}
	itemID, err := c.ParamsInt("item_id")
	if err != nil {
		_ = responses.BadRequest(c, "Invalid item ID")
		return nil, nil, true
	}
	item, err := h.svc.GetBankItemInBank(c.Context(), bank.ID, uint(itemID))
	if err != nil {
		_ = responses.NotFound(c, "bank item")
		return nil, nil, true
	}
	return bank, item, false
}

func (h *QuizItemBankHandler) ListBankItems(c *fiber.Ctx) error {
	bank, wrote := h.scopedItemBank(c)
	if wrote {
		return nil
	}
	items, err := h.svc.ListBankItems(c.Context(), bank.ID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch bank items")
	}
	out := make([]fiber.Map, len(items))
	for i := range items {
		out[i] = quizItemBankItemToJSON(&items[i])
	}
	return c.JSON(out)
}

func (h *QuizItemBankHandler) CreateBankItem(c *fiber.Ctx) error {
	bank, wrote := h.scopedItemBank(c)
	if wrote {
		return nil
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
		BankID:            bank.ID,
		Position:          input.Item.Position,
		QuestionType:      input.Item.QuestionType,
		QuestionText:      service.SanitizeHTML(input.Item.QuestionText),
		PointsPossible:    input.Item.PointsPossible,
		Answers:           input.Item.Answers,
		CorrectComments:   service.SanitizeHTML(input.Item.CorrectComments),
		IncorrectComments: service.SanitizeHTML(input.Item.IncorrectComments),
		NeutralComments:   service.SanitizeHTML(input.Item.NeutralComments),
	}
	if err := h.svc.CreateBankItem(c.Context(), item); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizItemBankItemToJSON(item))
}

func (h *QuizItemBankHandler) GetBankItem(c *fiber.Ctx) error {
	_, item, wrote := h.scopedBankItem(c)
	if wrote {
		return nil
	}
	return c.JSON(quizItemBankItemToJSON(item))
}

func (h *QuizItemBankHandler) UpdateBankItem(c *fiber.Ctx) error {
	_, item, wrote := h.scopedBankItem(c)
	if wrote {
		return nil
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
		item.QuestionText = service.SanitizeHTML(*input.Item.QuestionText)
	}
	if input.Item.PointsPossible != nil {
		item.PointsPossible = input.Item.PointsPossible
	}
	if input.Item.Answers != nil {
		item.Answers = *input.Item.Answers
	}
	if input.Item.CorrectComments != nil {
		item.CorrectComments = service.SanitizeHTML(*input.Item.CorrectComments)
	}
	if input.Item.IncorrectComments != nil {
		item.IncorrectComments = service.SanitizeHTML(*input.Item.IncorrectComments)
	}
	if input.Item.NeutralComments != nil {
		item.NeutralComments = service.SanitizeHTML(*input.Item.NeutralComments)
	}
	if err := h.svc.UpdateBankItem(c.Context(), item); err != nil {
		return responses.InternalError(c, "Could not update bank item")
	}
	return c.JSON(quizItemBankItemToJSON(item))
}

func (h *QuizItemBankHandler) DeleteBankItem(c *fiber.Ctx) error {
	_, item, wrote := h.scopedBankItem(c)
	if wrote {
		return nil
	}
	if err := h.svc.DeleteBankItem(c.Context(), item.ID); err != nil {
		return responses.NotFound(c, "bank item")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// ---------- Quiz integration ----------

// AddBankItemToQuiz copies a bank item into the given quiz as a QuizQuestion.
// POST /api/v1/courses/:course_id/quiz_item_banks/:bank_id/items/:item_id/add_to_quiz/:quiz_id
// The quiz must live in the bank's course and the caller's tenant (B133).
func (h *QuizItemBankHandler) AddBankItemToQuiz(c *fiber.Ctx) error {
	bank, item, wrote := h.scopedBankItem(c)
	if wrote {
		return nil
	}
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	if err := h.svc.VerifyQuizInCourse(c.Context(), uint(quizID), bank.CourseID, callerAccountID(c)); err != nil {
		return responses.NotFound(c, "quiz")
	}
	var input struct {
		Position int `json:"position"`
	}
	_ = c.BodyParser(&input)
	q, err := h.svc.AddBankItemToQuiz(c.Context(), item.ID, uint(quizID), input.Position)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(quizQuestionToJSON(q))
}

// RandomDraw draws N random items from a bank and adds them to the quiz.
// POST /api/v1/courses/:course_id/quiz_item_banks/:bank_id/random_draw
// body {"quiz_id": 9, "count": 5}
func (h *QuizItemBankHandler) RandomDraw(c *fiber.Ctx) error {
	bank, wrote := h.scopedItemBank(c)
	if wrote {
		return nil
	}
	var input struct {
		QuizID   uint `json:"quiz_id"`
		Count    int  `json:"count"`
		Position int  `json:"position"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.QuizID == 0 {
		return responses.BadRequest(c, "quiz_id is required")
	}
	if err := h.svc.VerifyQuizInCourse(c.Context(), input.QuizID, bank.CourseID, callerAccountID(c)); err != nil {
		return responses.NotFound(c, "quiz")
	}
	questions, err := h.svc.DrawItemsIntoQuiz(c.Context(), bank.ID, input.QuizID, input.Count, input.Position)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	out := make([]fiber.Map, len(questions))
	for i := range questions {
		out[i] = quizQuestionToJSON(&questions[i])
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}
