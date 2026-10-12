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
	app, br, ir, qr, _ := setupItemBankHandlerFull()
	return app, br, ir, qr
}

func setupItemBankHandlerFull() (
	*fiber.App,
	*mocks.MockQuizItemBankRepository,
	*mocks.MockQuizItemBankItemRepository,
	*mocks.MockQuizQuestionRepository,
	*mocks.MockQuizRepository,
) {
	br := new(mocks.MockQuizItemBankRepository)
	ir := new(mocks.MockQuizItemBankItemRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	quizRepo := new(mocks.MockQuizRepository)

	svc := service.NewQuizItemBankService(br, ir, qr)
	svc.SetQuizRepo(quizRepo)
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

	api.Get("/courses/:course_id/quiz_item_banks/:bank_id/items", h.ListBankItems)
	api.Post("/courses/:course_id/quiz_item_banks/:bank_id/items", h.CreateBankItem)
	api.Get("/courses/:course_id/quiz_item_banks/:bank_id/items/:item_id", h.GetBankItem)
	api.Put("/courses/:course_id/quiz_item_banks/:bank_id/items/:item_id", h.UpdateBankItem)
	api.Delete("/courses/:course_id/quiz_item_banks/:bank_id/items/:item_id", h.DeleteBankItem)
	api.Post("/courses/:course_id/quiz_item_banks/:bank_id/items/:item_id/add_to_quiz/:quiz_id", h.AddBankItemToQuiz)
	api.Post("/courses/:course_id/quiz_item_banks/:bank_id/random_draw", h.RandomDraw)

	return app, br, ir, qr, quizRepo
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
	app, br, ir, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1, CourseID: 1}, nil)
	ir.On("ListByBankID", mock.Anything, uint(1)).Return([]models.QuizItemBankItem{{ID: 1, BankID: 1}}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/quiz_item_banks/1/items", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 1)
}

// B032: a bank in another course (and so another tenant) is invisible —
// its items (answer keys) cannot be listed through a course the caller
// teaches.
func TestListBankItems_CrossCourseBank_404(t *testing.T) {
	app, br, ir, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(5)).Return(&models.QuizItemBank{ID: 5, CourseID: 999}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/quiz_item_banks/5/items", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	ir.AssertNotCalled(t, "ListByBankID", mock.Anything, mock.Anything)
}

// B032: an item id from another bank cannot be read, updated or deleted
// through a bank the caller owns.
func TestBankItem_ItemFromOtherBank_404(t *testing.T) {
	app, br, ir, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1, CourseID: 1}, nil)
	ir.On("FindByID", mock.Anything, uint(50)).Return(&models.QuizItemBankItem{ID: 50, BankID: 77}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/1/quiz_item_banks/1/items/50", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	body := testutil.JSONBody(map[string]interface{}{"item": map[string]interface{}{"question_text": "pwned"}})
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/1/quiz_item_banks/1/items/50", body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = testutil.MakeRequest(app, http.MethodDelete, "/courses/1/quiz_item_banks/1/items/50", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	ir.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	ir.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestCreateBankItem_Success(t *testing.T) {
	app, br, ir, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1, CourseID: 1}, nil)
	ir.On("Create", mock.Anything, mock.MatchedBy(func(i *models.QuizItemBankItem) bool { return i.BankID == 1 })).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"item": map[string]interface{}{
			"question_type": "essay",
			"question_text": "Discuss",
		},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks/1/items", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestCreateBankItem_CrossCourseBank_404(t *testing.T) {
	app, br, ir, _ := setupItemBankHandler()
	br.On("FindByID", mock.Anything, uint(5)).Return(&models.QuizItemBank{ID: 5, CourseID: 999}, nil)
	body := testutil.JSONBody(map[string]interface{}{"item": map[string]interface{}{"question_type": "essay", "question_text": "x"}})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks/5/items", body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	ir.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestAddBankItemToQuiz_SetsBankItemID(t *testing.T) {
	app, br, ir, qr, quizRepo := setupItemBankHandlerFull()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1, CourseID: 1}, nil)
	ir.On("FindByID", mock.Anything, uint(2)).Return(&models.QuizItemBankItem{ID: 2, BankID: 1, QuestionType: "essay", QuestionText: "Q?"}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(&models.Quiz{ID: 9, CourseID: 1}, nil)
	qr.On("Create", mock.Anything, mock.MatchedBy(func(q *models.QuizQuestion) bool {
		return q.QuizID == 9 && q.BankItemID != nil && *q.BankItemID == 2
	})).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{"position": 1})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks/1/items/2/add_to_quiz/9", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	out, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, float64(2), out["bank_item_id"])
}

// B133: the destination quiz must be in the bank's course and the
// caller's tenant.
func TestAddBankItemToQuiz_QuizInOtherCourse_404(t *testing.T) {
	app, br, ir, qr, quizRepo := setupItemBankHandlerFull()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1, CourseID: 1}, nil)
	ir.On("FindByID", mock.Anything, uint(2)).Return(&models.QuizItemBankItem{ID: 2, BankID: 1, QuestionType: "essay", QuestionText: "Q?"}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(&models.Quiz{ID: 9, CourseID: 42}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks/1/items/2/add_to_quiz/9", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	qr.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B133: random draw persists the drawn items into the target quiz.
func TestRandomDraw_PersistsIntoQuiz(t *testing.T) {
	app, br, ir, qr, quizRepo := setupItemBankHandlerFull()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1, CourseID: 1}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(&models.Quiz{ID: 9, CourseID: 1}, nil)
	pool := []models.QuizItemBankItem{
		{ID: 1, BankID: 1, QuestionType: "essay", QuestionText: "A"},
		{ID: 2, BankID: 1, QuestionType: "essay", QuestionText: "B"},
		{ID: 3, BankID: 1, QuestionType: "essay", QuestionText: "C"},
	}
	ir.On("ListByBankID", mock.Anything, uint(1)).Return(pool, nil)
	qr.On("Create", mock.Anything, mock.MatchedBy(func(q *models.QuizQuestion) bool {
		return q.QuizID == 9 && q.BankItemID != nil
	})).Return(nil).Twice()

	body := testutil.JSONBody(map[string]interface{}{"quiz_id": 9, "count": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks/1/random_draw", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 2)
	qr.AssertExpectations(t)
}

func TestRandomDraw_RequiresQuizID(t *testing.T) {
	app, br, _, qr, _ := setupItemBankHandlerFull()
	br.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizItemBank{ID: 1, CourseID: 1}, nil)
	body := testutil.JSONBody(map[string]interface{}{"count": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/1/quiz_item_banks/1/random_draw", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	qr.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// keep the linter from yelling when only one path uses repository directly
var _ = repository.PaginationParams{}
