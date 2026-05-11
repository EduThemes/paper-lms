package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRegister_Success(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	// FindByEmail returns nil (no existing user)
	mockRepo.On("FindByEmail", ctx, "john@example.com").Return(nil, errors.New("not found"))

	// Create succeeds
	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.User")).Return(nil)

	user, err := svc.Register(ctx, "John Doe", "john@example.com", "password123")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "John Doe", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, "john@example.com", user.LoginID)
	assert.Equal(t, "John Doe", user.ShortName)
	assert.Equal(t, "Doe, John", user.SortableName)
	assert.NotEmpty(t, user.PasswordHash)
	mockRepo.AssertExpectations(t)
}

func TestRegister_MissingFields(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	// Empty name
	user, err := svc.Register(ctx, "", "john@example.com", "password123")
	assert.Nil(t, user)
	assert.EqualError(t, err, "name, email, and password are required")

	// Empty email
	user, err = svc.Register(ctx, "John Doe", "", "password123")
	assert.Nil(t, user)
	assert.EqualError(t, err, "name, email, and password are required")

	// Empty password
	user, err = svc.Register(ctx, "John Doe", "john@example.com", "")
	assert.Nil(t, user)
	assert.EqualError(t, err, "name, email, and password are required")

	mockRepo.AssertExpectations(t)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	existingUser := &models.User{
		ID:    1,
		Name:  "Existing User",
		Email: "john@example.com",
	}
	mockRepo.On("FindByEmail", ctx, "john@example.com").Return(existingUser, nil)

	user, err := svc.Register(ctx, "John Doe", "john@example.com", "password123")

	assert.Nil(t, user)
	assert.EqualError(t, err, "user already exists")
	mockRepo.AssertExpectations(t)
}

func TestRegister_SortableName_SingleName(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	mockRepo.On("FindByEmail", ctx, "john@example.com").Return(nil, errors.New("not found"))
	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.User")).Return(nil)

	user, err := svc.Register(ctx, "John", "john@example.com", "password123")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "John", user.SortableName)
	mockRepo.AssertExpectations(t)
}

func TestAuthenticate_ByLoginID(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	user := &models.User{
		ID:      1,
		Name:    "John Doe",
		LoginID: "jdoe",
		Email:   "john@example.com",
	}
	// Set a real bcrypt hash so CheckPassword works
	err := user.HashPassword("password123")
	assert.NoError(t, err)

	mockRepo.On("FindByLoginID", ctx, "jdoe").Return(user, nil)

	result, err := svc.Authenticate(ctx, "jdoe", "password123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(1), result.ID)
	assert.Equal(t, "John Doe", result.Name)
	mockRepo.AssertExpectations(t)
}

func TestAuthenticate_FallbackToEmail(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	user := &models.User{
		ID:      2,
		Name:    "Jane Smith",
		LoginID: "jsmith",
		Email:   "jane@example.com",
	}
	err := user.HashPassword("password123")
	assert.NoError(t, err)

	// FindByLoginID fails, triggering email fallback
	mockRepo.On("FindByLoginID", ctx, "jane@example.com").Return(nil, errors.New("not found"))
	mockRepo.On("FindByEmail", ctx, "jane@example.com").Return(user, nil)

	result, err := svc.Authenticate(ctx, "jane@example.com", "password123")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(2), result.ID)
	assert.Equal(t, "Jane Smith", result.Name)
	mockRepo.AssertExpectations(t)
}

func TestAuthenticate_InvalidPassword(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	user := &models.User{
		ID:      1,
		Name:    "John Doe",
		LoginID: "jdoe",
		Email:   "john@example.com",
	}
	err := user.HashPassword("password123")
	assert.NoError(t, err)

	mockRepo.On("FindByLoginID", ctx, "jdoe").Return(user, nil)

	result, err := svc.Authenticate(ctx, "jdoe", "wrongpassword")

	assert.Nil(t, result)
	assert.EqualError(t, err, "invalid credentials")
	mockRepo.AssertExpectations(t)
}

func TestAuthenticate_UserNotFound(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	// Both lookups fail
	mockRepo.On("FindByLoginID", ctx, "nobody").Return(nil, errors.New("not found"))
	mockRepo.On("FindByEmail", ctx, "nobody").Return(nil, errors.New("not found"))

	result, err := svc.Authenticate(ctx, "nobody", "password123")

	assert.Nil(t, result)
	assert.EqualError(t, err, "invalid credentials")
	mockRepo.AssertExpectations(t)
}

func TestGetByID_Success(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	expectedUser := &models.User{
		ID:    42,
		Name:  "Alice Wonderland",
		Email: "alice@example.com",
	}
	mockRepo.On("FindByID", ctx, uint(42)).Return(expectedUser, nil)

	user, err := svc.GetByID(ctx, 42)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, uint(42), user.ID)
	assert.Equal(t, "Alice Wonderland", user.Name)
	mockRepo.AssertExpectations(t)
}

func TestUpdate_Success(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	user := &models.User{
		ID:    1,
		Name:  "John Updated",
		Email: "john.updated@example.com",
	}
	mockRepo.On("Update", ctx, user).Return(nil)

	err := svc.Update(ctx, user)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestList_Success(t *testing.T) {
	mockRepo := new(mocks.MockUserRepository)
	svc := service.NewUserService(mockRepo)

	ctx := context.Background()

	params := repository.PaginationParams{Page: 1, PerPage: 10}
	expectedResult := &repository.PaginatedResult[models.User]{
		Items: []models.User{
			{ID: 1, Name: "User One", Email: "one@example.com"},
			{ID: 2, Name: "User Two", Email: "two@example.com"},
		},
		TotalCount: 2,
		Page:       1,
		PerPage:    10,
	}
	mockRepo.On("List", ctx, params).Return(expectedResult, nil)

	result, err := svc.List(ctx, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Equal(t, "User One", result.Items[0].Name)
	assert.Equal(t, "User Two", result.Items[1].Name)
	mockRepo.AssertExpectations(t)
}
