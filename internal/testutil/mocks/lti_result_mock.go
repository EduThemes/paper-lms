package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// MockLTIResultRepository mocks repository.LTIResultRepository.
//
// Note: LTIResultRepository has no Delete method by design (LTI 1.3
// Results are append-only from the platform side; the tool re-posts a
// score to update). F-012 widening therefore did NOT touch this repo.
type MockLTIResultRepository struct {
	mock.Mock
}

func (m *MockLTIResultRepository) Create(ctx context.Context, result *models.LTIResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *MockLTIResultRepository) FindByID(ctx context.Context, id uint) (*models.LTIResult, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LTIResult), args.Error(1)
}

func (m *MockLTIResultRepository) Upsert(ctx context.Context, result *models.LTIResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *MockLTIResultRepository) ListByLineItem(ctx context.Context, lineItemID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTIResult], error) {
	args := m.Called(ctx, lineItemID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.LTIResult]), args.Error(1)
}
