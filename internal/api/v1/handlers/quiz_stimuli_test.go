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

func setupStimulusHandler() (*fiber.App, *mocks.MockQuizStimulusRepository, *mocks.MockQuizQuestionRepository) {
	sr := new(mocks.MockQuizStimulusRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	svc := service.NewQuizStimulusService(sr, qr)
	h := handlers.NewQuizStimulusHandler(svc)

	app := testutil.SetupTestApp()

	app.Post("/courses/:course_id/quiz_stimuli", h.CreateStimulus)
	app.Get("/courses/:course_id/quiz_stimuli/:stimulus_id", h.GetStimulus)
	app.Put("/courses/:course_id/quiz_stimuli/:stimulus_id", h.UpdateStimulus)
	app.Delete("/courses/:course_id/quiz_stimuli/:stimulus_id", h.DeleteStimulus)
	app.Post("/quiz_stimuli/:stimulus_id/questions/:question_id", h.LinkQuestion)
	app.Delete("/quiz_stimuli/:stimulus_id/questions/:question_id", h.UnlinkQuestion)
	app.Get("/quiz_stimuli/:stimulus_id/questions", h.ListQuestions)

	return app, sr, qr
}

func TestCreateStimulus_Success(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("Create", mock.Anything, mock.MatchedBy(func(s *models.QuizStimulus) bool {
		return s.CourseID == 5 && s.Title == "Passage"
	})).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"stimulus": map[string]string{"title": "Passage", "content": "{}"},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/5/quiz_stimuli", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestCreateStimulus_MissingTitle(t *testing.T) {
	app, _, _ := setupStimulusHandler()
	body := testutil.JSONBody(map[string]interface{}{"stimulus": map[string]string{}})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/5/quiz_stimuli", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetStimulus_CrossCourse(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/6/quiz_stimuli/1", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetStimulus_NotFound(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(nil, errors.New("nope"))
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/5/quiz_stimuli/1", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateStimulus_PartialPatch(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5, Title: "Old"}, nil).Twice()
	sr.On("Update", mock.Anything, mock.MatchedBy(func(s *models.QuizStimulus) bool {
		return s.Title == "New"
	})).Return(nil)
	body := testutil.JSONBody(map[string]interface{}{"stimulus": map[string]string{"title": "New"}})
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/5/quiz_stimuli/1", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteStimulus_Success(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
	sr.On("Delete", mock.Anything, uint(1)).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/5/quiz_stimuli/1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLinkQuestion_Success(t *testing.T) {
	app, sr, qr := setupStimulusHandler()
	qr.On("FindByID", mock.Anything, uint(7)).Return(&models.QuizQuestion{ID: 7}, nil)
	sr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizStimulus{ID: 3}, nil)
	sr.On("SetQuestionStimulus", mock.Anything, uint(7), mock.MatchedBy(func(sid *uint) bool {
		return sid != nil && *sid == 3
	})).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/quiz_stimuli/3/questions/7", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUnlinkQuestion_Success(t *testing.T) {
	app, sr, qr := setupStimulusHandler()
	qr.On("FindByID", mock.Anything, uint(7)).Return(&models.QuizQuestion{ID: 7}, nil)
	sr.On("SetQuestionStimulus", mock.Anything, uint(7), (*uint)(nil)).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodDelete, "/quiz_stimuli/3/questions/7", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListQuestions_Success(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizStimulus{ID: 3}, nil)
	sr.On("ListQuestionsForStimulus", mock.Anything, uint(3)).Return([]models.QuizQuestion{{ID: 1}, {ID: 2}}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/quiz_stimuli/3/questions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 2)
}
