package handlers_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func setupItemBankHandler() (
	*fiber.App,
	*mocks.MockQuizItemBankRepository,
	*mocks.MockQuizItemBankItemRepository,
	*mocks.MockQuizQuestionRepository,
) {
	br := new(mocks.MockQuizItemBankRepository)
	ir := new(mocks.MockQuizItemBankItemRepository)
	qr := new(mocks.MockQuizQuestionRepository)

	svc := service.NewQuizItemBankService(br, ir, qr)
	h := handlers.NewQuizItemBankHandler(svc)

	app := testutil.SetupTestApp()
	api := app.Group("", func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", uint(1))
		return c.Next()
	})

	api.Get("/courses/:course_id/quiz_item_banks/:bank_id", h.GetBank)
	api.Post("/courses/:course_id/quiz_item_banks", h.CreateBank)
	api.Put("/courses/:course_id/quiz_item_banks/:bank_id", h.UpdateBank)
	api.Delete("/courses/:course_id/quiz_item_banks/:bank_id", h.DeleteBank)

	api.Get("/quiz_item_banks/:bank_id/items", h.ListBankItems)
	api.Post("/quiz_item_banks/:bank_id/items", h.CreateBankItem)
	api.Post("/quiz_item_banks/:bank_id/items/:item_id/add_to_quiz/:quiz_id", h.AddBankItemToQuiz)
	api.Post("/quiz_item_banks/:bank_id/random_draw", h.RandomDraw)

	return app, br, ir, qr
}

func TestCreateBank_Success(t *testing.T) {
	app, br, _, _ := setupItemBankHandler()
	br.On("Create", mock.Anything, mock.MatchedBy(func(b *models.QuizItemBank) bool {
		return b.CourseID == 1 && b.Title == "Algebra" && b.CreatedByUserID == 7
	})).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"bank": map[string]interface{}{"title": "Algebra"},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	out, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "Algebra", out["title"])
	br.AssertExpectations(t)
}

func TestCreateBank_MissingTitle(t *testing.T) {
	app, _, _, _ := setupItemBankHandler()
	body := testutil.JSONBody(map[string]interface{}{"bank": map[string]interface{}{}})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetBank_NotFound(t *testing.T) {
	app, br, _, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(99)).Return(nil, errors.New("nope"))
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/quiz_item_banks/99", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetBank_CrossCourse(t *testing.T) {
	app, br, _, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(5)).Return(&models.QuizItemBank{ID: 5, CourseID: 999}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/quiz_item_banks/5", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateBank_Success(t *testing.T) {
	app, br, _, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(5)).Return(&models.QuizItemBank{ID: 5, CourseID: 1, Title: "Old"}, nil).Twice()
	br.On("Update", mock.Anything, mock.MatchedBy(func(b *models.QuizItemBank) bool {
		return b.ID == 5 && b.Title == "New"
	})).Return(nil)
	body := testutil.JSONBody(map[string]interface{}{"bank": map[string]interface{}{"title": "New"}})
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/1/quiz_item_banks/5", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteBank_Success(t *testing.T) {
	app, br, _, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(5)).Return(&models.QuizItemBank{ID: 5, CourseID: 1}, nil)
	br.On("Delete", mock.Anything, uint(5)).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/1/quiz_item_banks/5", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListBankItems_Success(t *testing.T) {
	app, _, ir, _ := setupItemBankHandler()
	ir.On("ListByBankID", mock.Anything, uint(1)).Return([]models.QuizItemBankItem{{ID: 1, BankID: 1}}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/quiz_item_banks/1/items", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 1)
}

func TestCreateBankItem_Success(t *testing.T) {
	app, br, ir, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1}, nil)
	ir.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizItemBankItem")).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"item": map[string]interface{}{
			"question_type": "essay",
			"question_text": "Discuss",
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/quiz_item_banks/1/items", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAddBankItemToQuiz_SetsBankItemID(t *testing.T) {
	app, _, ir, qr := setupItemBankHandler()
	ir.On("FindByID", mock.Anything, uint(2)).Return(&models.QuizItemBankItem{ID: 2, QuestionType: "essay", QuestionText: "Q?"}, nil)
	qr.On("Create", mock.Anything, mock.MatchedBy(func(q *models.QuizQuestion) bool {
		return q.QuizID == 9 && q.BankItemID != nil && *q.BankItemID == 2
	})).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{"position": 1})
	resp := testutil.MakeRequest(app, http.MethodPost, "/quiz_item_banks/1/items/2/add_to_quiz/9", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	out, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, float64(2), out["bank_item_id"])
}

func TestRandomDraw_Success(t *testing.T) {
	app, _, ir, _ := setupItemBankHandler()
	pool := []models.QuizItemBankItem{
		{ID: 1, QuestionType: "essay", QuestionText: "A"},
		{ID: 2, QuestionType: "essay", QuestionText: "B"},
		{ID: 3, QuestionType: "essay", QuestionText: "C"},
	}
	ir.On("ListByBankID", mock.Anything, uint(1)).Return(pool, nil)

	body := testutil.JSONBody(map[string]interface{}{"count": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, "/quiz_item_banks/1/random_draw", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 2)
}

// keep the linter from yelling when only one path uses repository directly
var _ = repository.PaginationParams{}
