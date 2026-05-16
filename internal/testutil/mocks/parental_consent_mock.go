package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockParentalConsentRepository mocks the postgres.ParentalConsentRepository
// interface. Lives in testutil/mocks so handler tests can share it across
// packages — the postgres concrete type can stay in its own package.
type MockParentalConsentRepository struct {
	mock.Mock
}

func (m *MockParentalConsentRepository) Create(ctx context.Context, consent *models.ParentalConsent) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

func (m *MockParentalConsentRepository) FindByID(ctx context.Context, id uint) (*models.ParentalConsent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ParentalConsent), args.Error(1)
}

func (m *MockParentalConsentRepository) FindByStudentID(ctx context.Context, studentID uint) ([]models.ParentalConsent, error) {
	args := m.Called(ctx, studentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ParentalConsent), args.Error(1)
}

func (m *MockParentalConsentRepository) FindByToken(ctx context.Context, token string) (*models.ParentalConsent, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ParentalConsent), args.Error(1)
}

func (m *MockParentalConsentRepository) Update(ctx context.Context, consent *models.ParentalConsent) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

func (m *MockParentalConsentRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ParentalConsent], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.ParentalConsent]), args.Error(1)
}

// MockAgeVerificationRepository mocks the postgres.AgeVerificationRepository
// interface.
type MockAgeVerificationRepository struct {
	mock.Mock
}

func (m *MockAgeVerificationRepository) Create(ctx context.Context, verification *models.AgeVerification) error {
	args := m.Called(ctx, verification)
	return args.Error(0)
}

func (m *MockAgeVerificationRepository) FindByUserID(ctx context.Context, userID uint) (*models.AgeVerification, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AgeVerification), args.Error(1)
}

func (m *MockAgeVerificationRepository) Update(ctx context.Context, verification *models.AgeVerification) error {
	args := m.Called(ctx, verification)
	return args.Error(0)
}
