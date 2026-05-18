package handlers_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupUserHandler creates a UserHandler wired to a mock repository and
// registers routes on a fresh test Fiber app. It returns the app and mock
// so each test can configure expectations independently.
func setupUserHandler() (*fiber.App, *mocks.MockUserRepository) {
	mockRepo := new(mocks.MockUserRepository)
	userService := service.NewUserService(mockRepo)
	handler := handlers.NewUserHandler(userService, "test-jwt-secret", "test", nil, nil, nil)

	app := testutil.SetupTestApp()

	// Public routes (no auth middleware needed)
	app.Post("/login", handler.Login)
	app.Post("/register", handler.Register)

	// Protected routes: inject user_id into Locals to simulate auth middleware
	protected := app.Group("", func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(1))
		c.Locals("account_id", uint(1))
		return c.Next()
	}, middleware.PaginationParams())

	protected.Get("/users/self", handler.GetSelf)
	protected.Get("/users/:id", handler.GetUser)
	protected.Put("/users/:id", handler.UpdateUser)
	protected.Get("/users", handler.ListUsers)

	return app, mockRepo
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	app, mockRepo := setupUserHandler()

	user := &models.User{
		ID:      1,
		Name:    "John Doe",
		LoginID: "john@example.com",
		Email:   "john@example.com",
		Locale:  "en",
	}
	err := user.HashPassword("password123")
	assert.NoError(t, err)

	mockRepo.On("FindByLoginID", mock.Anything, "john@example.com").Return(user, nil)

	body := testutil.JSONBody(map[string]string{
		"email":    "john@example.com",
		"password": "password123",
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/login", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, result["token"])
	assert.NotNil(t, result["user"])

	userMap := result["user"].(map[string]interface{})
	assert.Equal(t, "John Doe", userMap["name"])
	assert.Equal(t, "john@example.com", userMap["email"])

	mockRepo.AssertExpectations(t)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	app, mockRepo := setupUserHandler()

	user := &models.User{
		ID:      1,
		Name:    "John Doe",
		LoginID: "john@example.com",
		Email:   "john@example.com",
	}
	err := user.HashPassword("password123")
	assert.NoError(t, err)

	mockRepo.On("FindByLoginID", mock.Anything, "john@example.com").Return(user, nil)

	body := testutil.JSONBody(map[string]string{
		"email":    "john@example.com",
		"password": "wrongpassword",
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/login", body)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])

	mockRepo.AssertExpectations(t)
}

func TestLogin_UserNotFound(t *testing.T) {
	app, mockRepo := setupUserHandler()

	mockRepo.On("FindByLoginID", mock.Anything, "nobody@example.com").Return(nil, errors.New("not found"))
	mockRepo.On("FindByEmail", mock.Anything, "nobody@example.com").Return(nil, errors.New("not found"))

	body := testutil.JSONBody(map[string]string{
		"email":    "nobody@example.com",
		"password": "password123",
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/login", body)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	mockRepo.AssertExpectations(t)
}

func TestLogin_BadJSON(t *testing.T) {
	app, _ := setupUserHandler()

	resp := testutil.MakeRequest(app, http.MethodPost, "/login", testutil.JSONBody("not json"))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func TestRegister_Success(t *testing.T) {
	app, mockRepo := setupUserHandler()

	mockRepo.On("FindByEmail", mock.Anything, "jane@example.com").Return(nil, errors.New("not found"))
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

	body := testutil.JSONBody(map[string]string{
		"name":     "Jane Smith",
		"email":    "jane@example.com",
		"password": "password123",
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/register", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotEmpty(t, result["token"])
	assert.NotNil(t, result["user"])

	userMap := result["user"].(map[string]interface{})
	assert.Equal(t, "Jane Smith", userMap["name"])
	assert.Equal(t, "jane@example.com", userMap["email"])

	mockRepo.AssertExpectations(t)
}

func TestRegister_Duplicate(t *testing.T) {
	app, mockRepo := setupUserHandler()

	existingUser := &models.User{
		ID:    1,
		Name:  "Existing User",
		Email: "jane@example.com",
	}
	mockRepo.On("FindByEmail", mock.Anything, "jane@example.com").Return(existingUser, nil)

	body := testutil.JSONBody(map[string]string{
		"name":     "Jane Smith",
		"email":    "jane@example.com",
		"password": "password123",
	})

	resp := testutil.MakeRequest(app, http.MethodPost, "/register", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])

	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// GetUser
// ---------------------------------------------------------------------------

func TestGetUser_Success(t *testing.T) {
	app, mockRepo := setupUserHandler()

	user := &models.User{
		ID:           1,
		Name:         "John Doe",
		SortableName: "Doe, John",
		ShortName:    "John Doe",
		LoginID:      "john@example.com",
		Email:        "john@example.com",
		Locale:       "en",
		TimeZone:     "America/New_York",
	}
	mockRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(user, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/users/1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), result["id"])
	assert.Equal(t, "John Doe", result["name"])
	assert.Equal(t, "john@example.com", result["email"])
	assert.Equal(t, "Doe, John", result["sortable_name"])

	mockRepo.AssertExpectations(t)
}

func TestGetUser_NotFound(t *testing.T) {
	app, mockRepo := setupUserHandler()

	mockRepo.On("FindByID", mock.Anything, uint(999), uint(1)).Return(nil, errors.New("not found"))

	resp := testutil.MakeRequest(app, http.MethodGet, "/users/999", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])

	mockRepo.AssertExpectations(t)
}

func TestGetUser_InvalidID(t *testing.T) {
	app, _ := setupUserHandler()

	resp := testutil.MakeRequest(app, http.MethodGet, "/users/abc", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])
}

// ---------------------------------------------------------------------------
// GetSelf
// ---------------------------------------------------------------------------

func TestGetSelf(t *testing.T) {
	app, mockRepo := setupUserHandler()

	user := &models.User{
		ID:           1,
		Name:         "John Doe",
		SortableName: "Doe, John",
		ShortName:    "John Doe",
		LoginID:      "john@example.com",
		Email:        "john@example.com",
		Locale:       "en",
		TimeZone:     "America/New_York",
	}
	// The middleware sets user_id = 1, so GetSelf calls GetByID(1).
	// GetSelf passes accountID=0 (self lookup; userID IS the caller).
	mockRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(user, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/users/self", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), result["id"])
	assert.Equal(t, "John Doe", result["name"])
	assert.Equal(t, "john@example.com", result["email"])

	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// UpdateUser
// ---------------------------------------------------------------------------

func TestUpdateUser(t *testing.T) {
	app, mockRepo := setupUserHandler()

	user := &models.User{
		ID:           1,
		Name:         "John Doe",
		SortableName: "Doe, John",
		ShortName:    "John Doe",
		LoginID:      "john@example.com",
		Email:        "john@example.com",
		Locale:       "en",
		TimeZone:     "America/New_York",
	}
	mockRepo.On("FindByID", mock.Anything, uint(1), uint(1)).Return(user, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"user": map[string]string{
			"name":   "John Updated",
			"locale": "es",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPut, "/users/1", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", result["name"])
	assert.Equal(t, "es", result["locale"])

	mockRepo.AssertExpectations(t)
}

func TestUpdateUser_Forbidden(t *testing.T) {
	app, mockRepo := setupUserHandler()

	// The middleware injects user_id = 1, but we try to update user 2
	// The handler should return 403 before calling FindByID
	body := testutil.JSONBody(map[string]interface{}{
		"user": map[string]string{
			"name": "Hacker",
		},
	})

	resp := testutil.MakeRequest(app, http.MethodPut, "/users/2", body)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	result, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.NotNil(t, result["errors"])

	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ListUsers
// ---------------------------------------------------------------------------

func TestListUsers(t *testing.T) {
	app, mockRepo := setupUserHandler()

	expectedResult := &repository.PaginatedResult[models.User]{
		Items: []models.User{
			{ID: 1, Name: "User One", Email: "one@example.com", LoginID: "one@example.com", Locale: "en"},
			{ID: 2, Name: "User Two", Email: "two@example.com", LoginID: "two@example.com", Locale: "en"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}
	mockRepo.On("List", mock.Anything, repository.PaginationParams{Page: 1, PerPage: 10}, uint(1)).Return(expectedResult, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/users", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify Link pagination header is set
	assert.NotEmpty(t, resp.Header.Get("Link"))

	result, err := testutil.ParseJSONArray(resp)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "User One", result[0]["name"])
	assert.Equal(t, "User Two", result[1]["name"])

	mockRepo.AssertExpectations(t)
}
