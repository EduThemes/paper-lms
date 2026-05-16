package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockContentEmbeddingRepository mocks repository.ContentEmbeddingRepository.
type MockContentEmbeddingRepository struct {
	mock.Mock
}

func (m *MockContentEmbeddingRepository) Upsert(ctx context.Context, e *models.ContentEmbedding) error {
	return m.Called(ctx, e).Error(0)
}

func (m *MockContentEmbeddingRepository) DeleteByContent(ctx context.Context, contentType string, contentID uint) error {
	return m.Called(ctx, contentType, contentID).Error(0)
}

func (m *MockContentEmbeddingRepository) SearchByCourse(ctx context.Context, courseID, accountID uint, queryVec []float32, limit int) ([]repository.SearchHit, error) {
	args := m.Called(ctx, courseID, accountID, queryVec, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.SearchHit), args.Error(1)
}

func (m *MockContentEmbeddingRepository) ListByCourse(ctx context.Context, courseID, accountID uint) ([]models.ContentEmbedding, error) {
	args := m.Called(ctx, courseID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ContentEmbedding), args.Error(1)
}
