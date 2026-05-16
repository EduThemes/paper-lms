package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockSubmissionCommentRepository mocks repository.SubmissionCommentRepository
type MockSubmissionCommentRepository struct {
	mock.Mock
}

func (m *MockSubmissionCommentRepository) Create(ctx context.Context, comment *models.SubmissionComment) error {
	args := m.Called(ctx, comment)
	return args.Error(0)
}

func (m *MockSubmissionCommentRepository) ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.SubmissionComment, error) {
	args := m.Called(ctx, submissionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.SubmissionComment), args.Error(1)
}
