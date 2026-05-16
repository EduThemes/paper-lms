package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockConferenceParticipantRepository mocks repository.ConferenceParticipantRepository
type MockConferenceParticipantRepository struct {
	mock.Mock
}

func (m *MockConferenceParticipantRepository) Create(ctx context.Context, participant *models.ConferenceParticipant) error {
	args := m.Called(ctx, participant)
	return args.Error(0)
}

func (m *MockConferenceParticipantRepository) FindByID(ctx context.Context, id, accountID uint) (*models.ConferenceParticipant, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConferenceParticipant), args.Error(1)
}

func (m *MockConferenceParticipantRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConferenceParticipantRepository) ListByConferenceID(ctx context.Context, conferenceID, accountID uint) ([]models.ConferenceParticipant, error) {
	args := m.Called(ctx, conferenceID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ConferenceParticipant), args.Error(1)
}

func (m *MockConferenceParticipantRepository) FindByConferenceAndUser(ctx context.Context, conferenceID, userID, accountID uint) (*models.ConferenceParticipant, error) {
	args := m.Called(ctx, conferenceID, userID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConferenceParticipant), args.Error(1)
}
