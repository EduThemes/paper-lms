package handlers_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// These tests lock the 2026-06-09 red-team cross-tenant IDOR fixes. The
// shared shape of the bug: a nested-route leaf (quiz question, wallet)
// was addressable by its own numeric id while the route middleware only
// guarded :course_id / the self-or-admin check — so a caller in one
// tenant could reach another tenant's resource. The fixes tie each leaf
// back to its parent (and thus the caller's tenant) and return an
// existence-leak-safe 404 on any mismatch.

// --- #1 Quiz-question nested-route IDOR ------------------------------------

// errRepoMiss stands in for a tenant-scoped repo lookup that misses
// (e.g. a quiz that belongs to another tenant).
var errRepoMiss = errors.New("not found")

func setupQuizQuestionHandler(accountID uint) (*fiber.App, *mocks.MockQuizRepository, *mocks.MockQuizQuestionRepository) {
	quizRepo := new(mocks.MockQuizRepository)
	questionRepo := new(mocks.MockQuizQuestionRepository)
	quizService := service.NewQuizService(quizRepo, questionRepo, nil, nil)
	h := handlers.NewQuizQuestionHandler(quizService)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(1))
		c.Locals("account_id", accountID)
		return c.Next()
	})
	app.Get("/api/v1/courses/:course_id/quizzes/:quiz_id/questions/:question_id", h.GetQuestion)
	return app, quizRepo, questionRepo
}

// A quiz that resolves within the caller's tenant but lives in a
// DIFFERENT course than the URL's :course_id must 404 — and the question
// must never be loaded.
func TestQuizQuestion_CrossCourse_Returns404(t *testing.T) {
	app, quizRepo, questionRepo := setupQuizQuestionHandler(1)
	// Quiz 5 belongs to course 999, not the URL's course 7.
	quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).
		Return(&models.Quiz{CourseID: 999}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/courses/7/quizzes/5/questions/100", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	questionRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

// A quiz that is not in the caller's tenant (the tenant-scoped lookup
// misses) must 404 before the question is loaded.
func TestQuizQuestion_CrossTenant_Returns404(t *testing.T) {
	app, quizRepo, questionRepo := setupQuizQuestionHandler(1)
	// GetQuizScoped(quizID, accountID=1) misses → another tenant's quiz.
	quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).
		Return(nil, errRepoMiss)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/courses/7/quizzes/5/questions/100", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	questionRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

// A question whose QuizID points at a different quiz than the URL's
// :quiz_id must 404 even though the quiz itself is in-course/in-tenant.
func TestQuizQuestion_WrongQuiz_Returns404(t *testing.T) {
	app, quizRepo, questionRepo := setupQuizQuestionHandler(1)
	quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).
		Return(&models.Quiz{CourseID: 7}, nil)
	// Question 100 actually belongs to quiz 6, not the URL's quiz 5.
	questionRepo.On("FindByID", mock.Anything, uint(100)).
		Return(&models.QuizQuestion{QuizID: 6}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/courses/7/quizzes/5/questions/100", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// Sanity: the fully-consistent chain (quiz in course+tenant, question in
// quiz) still succeeds — the fix is a tie, not a blanket deny.
func TestQuizQuestion_HappyPath_Returns200(t *testing.T) {
	app, quizRepo, questionRepo := setupQuizQuestionHandler(1)
	quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).
		Return(&models.Quiz{CourseID: 7}, nil)
	questionRepo.On("FindByID", mock.Anything, uint(100)).
		Return(&models.QuizQuestion{QuizID: 5}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/api/v1/courses/7/quizzes/5/questions/100", nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// --- #2 Cross-tenant wallet read -------------------------------------------

// An account-1 admin requesting a user that does NOT belong to account 1
// must get a 404, and the (non-tenant-scoped) balance repo must never be
// reached.
func TestGetUserWallet_CrossTenant_Returns404(t *testing.T) {
	app, walletRepo, _, userRepo, _, _ := setupGamificationHandler(99, true /*isAdmin*/)
	// Target user 42 is outside the caller's tenant → gate fails.
	userRepo.On("FindByID", mock.Anything, uint(42), uint(1)).
		Return(nil, gamification.ErrUserNotFound)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/users/42/wallet", nil)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	walletRepo.AssertNotCalled(t, "ListBalancesForUser", mock.Anything, mock.Anything)
}
