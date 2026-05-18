package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockPeerReviewRepository mocks repository.PeerReviewRepository
type MockPeerReviewRepository struct {
	mock.Mock
}

func (m *MockPeerReviewRepository) Create(ctx context.Context, pr *models.PeerReview) error {
	args := m.Called(ctx, pr)
	return args.Error(0)
}

func (m *MockPeerReviewRepository) FindByID(ctx context.Context, id, accountID uint) (*models.PeerReview, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PeerReview), args.Error(1)
}

func (m *MockPeerReviewRepository) Update(ctx context.Context, pr *models.PeerReview) error {
	args := m.Called(ctx, pr)
	return args.Error(0)
}

func (m *MockPeerReviewRepository) ListByAssignment(ctx context.Context, assignmentID, accountID uint) ([]models.PeerReview, error) {
	args := m.Called(ctx, assignmentID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PeerReview), args.Error(1)
}

func (m *MockPeerReviewRepository) ListByReviewer(ctx context.Context, assignmentID, reviewerID, accountID uint) ([]models.PeerReview, error) {
	args := m.Called(ctx, assignmentID, reviewerID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PeerReview), args.Error(1)
}

func (m *MockPeerReviewRepository) FindByAssignmentAndReviewerAndReviewee(ctx context.Context, assignmentID, reviewerID, revieweeID, accountID uint) (*models.PeerReview, error) {
	args := m.Called(ctx, assignmentID, reviewerID, revieweeID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PeerReview), args.Error(1)
}

func (m *MockPeerReviewRepository) DeleteByAssignment(ctx context.Context, assignmentID, accountID uint) error {
	args := m.Called(ctx, assignmentID, accountID)
	return args.Error(0)
}
