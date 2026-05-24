package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupSetupHandler wires the SetupHandler with mock repositories and the
// given bootstrap token. db is nil — the in-process mutex covers the test
// path; the advisory lock is exercised in production only.
func setupSetupHandler(bootstrapToken string) (*mocks.MockUserRepository, *mocks.MockAccountRepository, *handlers.SetupHandler) {
	userRepo := new(mocks.MockUserRepository)
	accountRepo := new(mocks.MockAccountRepository)
	userService := service.NewUserService(userRepo)
	h := handlers.NewSetupHandler(userService, accountRepo, userRepo, nil, "test-secret", "test", bootstrapToken)
	return userRepo, accountRepo, h
}

// emptyUserList returns the "no admin exists yet" hasAdmin response.
func emptyUserList() *repository.PaginatedResult[models.User] {
	return &repository.PaginatedResult[models.User]{
		Items:      []models.User{},
		TotalCount: 0,
		Page:       1,
		PerPage:    100,
	}
}

func validSetupBody() map[string]string {
	return map[string]string{
		"admin_name":     "Operator",
		"admin_email":    "ops@example.com",
		"admin_password": "supersecret",
	}
}

// TestCompleteSetup_NoToken_LegacyBehavior confirms that with
// SETUP_BOOTSTRAP_TOKEN unset, the wizard still works for backward
// compatibility (development mode).
func TestCompleteSetup_NoToken_LegacyBehavior(t *testing.T) {
	userRepo, _, h := setupSetupHandler("")

	userRepo.On("List", mock.Anything, mock.Anything, uint(0)).Return(emptyUserList(), nil)
	userRepo.On("FindByEmail", mock.Anything, "ops@example.com").Return(nil, nil)
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(1).(*models.User)
		u.ID = 1
	})
	userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

	app := testutil.SetupTestApp()
	app.Post("/setup/complete", h.CompleteSetup)

	req := httptest.NewRequest(http.MethodPost, "/setup/complete", testutil.JSONBody(validSetupBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCompleteSetup_WithToken_CorrectHeader admits the request.
func TestCompleteSetup_WithToken_CorrectHeader(t *testing.T) {
	const token = "correct-horse-battery-staple"
	userRepo, _, h := setupSetupHandler(token)

	userRepo.On("List", mock.Anything, mock.Anything, uint(0)).Return(emptyUserList(), nil)
	userRepo.On("FindByEmail", mock.Anything, "ops@example.com").Return(nil, nil)
	userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
		u := args.Get(1).(*models.User)
		u.ID = 1
	})
	userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

	app := testutil.SetupTestApp()
	app.Post("/setup/complete", h.CompleteSetup)

	req := httptest.NewRequest(http.MethodPost, "/setup/complete", testutil.JSONBody(validSetupBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Setup-Token", token)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCompleteSetup_WithToken_WrongHeader rejects with 401 and never
// touches the repository (verified by asserting no .On() expectations
// were used).
func TestCompleteSetup_WithToken_WrongHeader(t *testing.T) {
	const token = "correct-horse-battery-staple"
	userRepo, _, h := setupSetupHandler(token)

	app := testutil.SetupTestApp()
	app.Post("/setup/complete", h.CompleteSetup)

	req := httptest.NewRequest(http.MethodPost, "/setup/complete", testutil.JSONBody(validSetupBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Setup-Token", "wrong")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	userRepo.AssertNotCalled(t, "List")
	userRepo.AssertNotCalled(t, "Create")
}

// TestCompleteSetup_WithToken_MissingHeader rejects with 401.
func TestCompleteSetup_WithToken_MissingHeader(t *testing.T) {
	userRepo, _, h := setupSetupHandler("required-token")

	app := testutil.SetupTestApp()
	app.Post("/setup/complete", h.CompleteSetup)

	req := httptest.NewRequest(http.MethodPost, "/setup/complete", testutil.JSONBody(validSetupBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	userRepo.AssertNotCalled(t, "List")
}

// TestGetStatus_AlwaysPublic confirms that the bootstrap token does NOT
// apply to /setup/status — the frontend wizard reads this unauthenticated
// to know whether to render itself.
func TestGetStatus_AlwaysPublic(t *testing.T) {
	userRepo, _, h := setupSetupHandler("required-token")
	userRepo.On("List", mock.Anything, mock.Anything, uint(0)).Return(emptyUserList(), nil)

	app := testutil.SetupTestApp()
	app.Get("/setup/status", h.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/setup/status", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCompleteSetup_AlreadyCompleted returns 403 regardless of token —
// the hasAdmin recheck inside the lock catches the race where two
// callers both passed the token gate before either created the admin.
func TestCompleteSetup_AlreadyCompleted(t *testing.T) {
	userRepo, _, h := setupSetupHandler("")

	existing := &repository.PaginatedResult[models.User]{
		Items: []models.User{
			{ID: 1, Role: "super_admin"},
		},
		TotalCount: 1,
		Page:       1,
		PerPage:    100,
	}
	userRepo.On("List", mock.Anything, mock.Anything, uint(0)).Return(existing, nil)

	app := testutil.SetupTestApp()
	app.Post("/setup/complete", h.CompleteSetup)

	req := httptest.NewRequest(http.MethodPost, "/setup/complete", testutil.JSONBody(validSetupBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
