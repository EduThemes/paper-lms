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

// Fixture: tenant 1, course 10, quiz 5 (in course 10), question 1 (in quiz 5),
// outcome 2 (course 10's own outcome).
const alignBase = "/courses/10/quizzes/5/questions/1/outcome_alignments"

type alignmentMocks struct {
	ar   *mocks.MockQuizQuestionOutcomeAlignmentRepository
	qr   *mocks.MockQuizQuestionRepository
	or   *mocks.MockLearningOutcomeRepository
	quiz *mocks.MockQuizRepository
}

func setupAlignmentHandler() (*fiber.App, alignmentMocks) {
	m := alignmentMocks{
		ar:   new(mocks.MockQuizQuestionOutcomeAlignmentRepository),
		qr:   new(mocks.MockQuizQuestionRepository),
		or:   new(mocks.MockLearningOutcomeRepository),
		quiz: new(mocks.MockQuizRepository),
	}
	svc := service.NewQuizOutcomeAlignmentService(m.ar, m.qr, m.or).SetQuizRepo(m.quiz)
	h := handlers.NewQuizOutcomeAlignmentHandler(svc)

	app := testutil.SetupTestApp()
	app.Use(callerStub(7, 1))
	app.Post("/courses/:course_id/quizzes/:quiz_id/questions/:question_id/outcome_alignments", h.Align)
	app.Get("/courses/:course_id/quizzes/:quiz_id/questions/:question_id/outcome_alignments", h.ListByQuestion)
	app.Delete("/courses/:course_id/quizzes/:quiz_id/questions/:question_id/outcome_alignments/:outcome_id", h.Unalign)
	app.Get("/courses/:course_id/outcomes/:outcome_id/quiz_question_alignments", h.ListByOutcome)
	return app, m
}

// inCourse stubs quiz 5 in course 10 (tenant 1) holding question 1.
func (m alignmentMocks) inCourse() {
	m.quiz.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 10}, nil)
	m.qr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5}, nil)
}

func TestAlign_Success(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.inCourse()
	m.or.On("FindByID", mock.Anything, uint(2), uint(1)).Return(&models.LearningOutcome{ID: 2, ContextType: "Course", ContextID: 10}, nil)
	m.ar.On("FindByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(nil, errors.New("not found"))
	m.ar.On("Create", mock.Anything, mock.MatchedBy(func(a *models.QuizQuestionOutcomeAlignment) bool {
		return a.QuizQuestionID == 1 && a.OutcomeID == 2 && a.MasteryThreshold == 0.85
	})).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2, "mastery_threshold": 0.85})
	resp := testutil.MakeRequest(app, http.MethodPost, alignBase, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAlign_DefaultThreshold(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.inCourse()
	m.or.On("FindByID", mock.Anything, uint(2), uint(1)).Return(&models.LearningOutcome{ID: 2, ContextType: "Account", ContextID: 1}, nil)
	m.ar.On("FindByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(nil, errors.New("not found"))
	m.ar.On("Create", mock.Anything, mock.MatchedBy(func(a *models.QuizQuestionOutcomeAlignment) bool {
		return a.MasteryThreshold == 0.7
	})).Return(nil)
	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, alignBase, body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAlign_Duplicate(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.inCourse()
	m.or.On("FindByID", mock.Anything, uint(2), uint(1)).Return(&models.LearningOutcome{ID: 2, ContextType: "Course", ContextID: 10}, nil)
	m.ar.On("FindByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(&models.QuizQuestionOutcomeAlignment{ID: 99}, nil)
	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, alignBase, body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// F4: a quiz outside the caller's tenant (repo scope miss) is a 404 and
// nothing is written.
func TestAlign_CrossTenantQuizIsNotFound(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.quiz.On("FindByID", mock.Anything, uint(5), uint(1)).Return(nil, errors.New("record not found"))
	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, alignBase, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	m.ar.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// F4: a question of another quiz can't be reached through this quiz's URL.
func TestAlign_QuestionOfOtherQuizIsNotFound(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.quiz.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 10}, nil)
	m.qr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 99}, nil)
	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, alignBase, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	m.ar.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// F4: another course's outcome can't be aligned.
func TestAlign_OutcomeOfOtherCourseIsNotFound(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.inCourse()
	m.or.On("FindByID", mock.Anything, uint(2), uint(1)).Return(&models.LearningOutcome{ID: 2, ContextType: "Course", ContextID: 11}, nil)
	body := testutil.JSONBody(map[string]interface{}{"outcome_id": 2})
	resp := testutil.MakeRequest(app, http.MethodPost, alignBase, body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	m.ar.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestUnalign_Success(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.inCourse()
	m.ar.On("DeleteByQuestionAndOutcome", mock.Anything, uint(1), uint(2)).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodDelete, alignBase+"/2", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUnalign_CrossTenantIsNotFound(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.quiz.On("FindByID", mock.Anything, uint(5), uint(1)).Return(nil, errors.New("record not found"))
	resp := testutil.MakeRequest(app, http.MethodDelete, alignBase+"/2", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	m.ar.AssertNotCalled(t, "DeleteByQuestionAndOutcome", mock.Anything, mock.Anything, mock.Anything)
}

func TestListByQuestion_Success(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.inCourse()
	m.ar.On("ListByQuestionID", mock.Anything, uint(1)).Return([]models.QuizQuestionOutcomeAlignment{{ID: 1}}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, alignBase, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 1)
}

// ListByOutcome only returns alignments whose question's quiz is in the course.
func TestListByOutcome_FiltersToCourse(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.or.On("FindByID", mock.Anything, uint(2), uint(1)).Return(&models.LearningOutcome{ID: 2, ContextType: "Account", ContextID: 1}, nil)
	m.ar.On("ListByOutcomeID", mock.Anything, uint(2)).Return([]models.QuizQuestionOutcomeAlignment{
		{ID: 1, QuizQuestionID: 1, OutcomeID: 2},
		{ID: 2, QuizQuestionID: 3, OutcomeID: 2},
	}, nil)
	m.qr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5}, nil)
	m.qr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizQuestion{ID: 3, QuizID: 6}, nil)
	m.quiz.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 10}, nil)
	m.quiz.On("FindByID", mock.Anything, uint(6), uint(1)).Return(&models.Quiz{ID: 6, CourseID: 11}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/10/outcomes/2/quiz_question_alignments", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 1)
}

func TestListByOutcome_CrossTenantOutcomeIsNotFound(t *testing.T) {
	app, m := setupAlignmentHandler()
	m.or.On("FindByID", mock.Anything, uint(2), uint(1)).Return(nil, errors.New("record not found"))
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/10/outcomes/2/quiz_question_alignments", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
