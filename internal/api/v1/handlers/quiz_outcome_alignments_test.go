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
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func setupAlignmentHandler() (*fiber.App, *mocks.MockQuizQuestionOutcomeAlignmentRepository, *mocks.MockQuizQuestionRepository, *mocks.MockLearningOutcomeRepository) {
	ar := new(mocks.MockQuizQuestionOutcomeAlignmentRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	or := new(mocks.MockLearningOutcomeRepository)
	svc := service.NewQuizOutcomeAlignmentService(ar, qr, or)
	h := handlers.NewQuizOutcomeAlignmentHandler(svc)

	app := testutil.SetupTestApp()
	app.Post("/quiz_questions/:question_id/outcome_alignments", h.Align)
	app.Get("/quiz_questions/:question_id/outcome_alignments", h.ListByQuestion)
	app.Delete("/quiz_questions/:question_id/outcome_alignments/:outcome_id", h.Unalign)
	app.Get("/learning_outcomes/:outcome_id/quiz_question_alignments", h.ListByOutcome)
	return app, ar, qr, or
}

func TestAlign_Success(t *testing.T) {
	app, ar, qr, or := setupAlignmentHandler()
	qr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1}, nil)
	or.On("FindByID", mock.Anything, uint(2), uint(0)).Return(&models.LearningOutcome{ID: 2}, nil)
	ar.On("FindByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(nil, errors.New("not found"))
	ar.On("Create", mock.Anything, mock.MatchedBy(func(a *models.QuizQuestionOutcomeAlignment) bool {
		return a.QuizQuestionID == 1 && a.OutcomeID == 2 && a.MasteryThreshold == 0.85
	})).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2, "mastery_threshold": 0.85})
	resp := testutil.MakeRequest(app, http.MethodPost, "/quiz_questions/1/outcome_alignments", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAlign_DefaultThreshold(t *testing.T) {
	app, ar, qr, or := setupAlignmentHandler()
	qr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1}, nil)
	or.On("FindByID", mock.Anything, uint(2), uint(0)).Return(&models.LearningOutcome{ID: 2}, nil)
	ar.On("FindByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(nil, errors.New("not found"))
	ar.On("Create", mock.Anything, mock.MatchedBy(func(a *models.QuizQuestionOutcomeAlignment) bool {
		return a.MasteryThreshold == 0.7
	})).Return(nil)
	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, "/quiz_questions/1/outcome_alignments", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAlign_Duplicate(t *testing.T) {
	app, ar, qr, or := setupAlignmentHandler()
	qr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1}, nil)
	or.On("FindByID", mock.Anything, uint(2), uint(0)).Return(&models.LearningOutcome{ID: 2}, nil)
	ar.On("FindByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(&models.QuizQuestionOutcomeAlignment{ID: 99}, nil)
	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, "/quiz_questions/1/outcome_alignments", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUnalign_Success(t *testing.T) {
	app, ar, _, _ := setupAlignmentHandler()
	ar.On("DeleteByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodDelete, "/quiz_questions/1/outcome_alignments/2", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListByQuestion_Success(t *testing.T) {
	app, ar, _, _ := setupAlignmentHandler()
	ar.On("ListByQuestionID", mock.Anything, uint(1)).Return([]models.QuizQuestionOutcomeAlignment{{ID: 1}}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/quiz_questions/1/outcome_alignments", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 1)
}

func TestListByOutcome_Success(t *testing.T) {
	app, ar, _, _ := setupAlignmentHandler()
	ar.On("ListByOutcomeID", mock.Anything, uint(2)).Return([]models.QuizQuestionOutcomeAlignment{{ID: 1}, {ID: 2}}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/learning_outcomes/2/quiz_question_alignments", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 2)
}
