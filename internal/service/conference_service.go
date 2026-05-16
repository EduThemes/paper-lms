package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type ConferenceService struct {
	confRepo        repository.ConferenceRepository
	participantRepo repository.ConferenceParticipantRepository
}

func NewConferenceService(confRepo repository.ConferenceRepository, participantRepo repository.ConferenceParticipantRepository) *ConferenceService {
	return &ConferenceService{
		confRepo:        confRepo,
		participantRepo: participantRepo,
	}
}

// CRUD

func (s *ConferenceService) Create(ctx context.Context, conference *models.Conference) error {
	if conference.Title == "" {
		return errors.New("conference title is required")
	}
	if conference.ConferenceType == "" {
		return errors.New("conference_type is required")
	}
	if conference.ConferenceType != "BigBlueButton" && conference.ConferenceType != "Zoom" {
		return errors.New("conference_type must be 'BigBlueButton' or 'Zoom'")
	}
	if conference.ContextType == "" || conference.ContextID == 0 {
		return errors.New("context_type and context_id are required")
	}
	if conference.WorkflowState == "" {
		conference.WorkflowState = "active"
	}
	if conference.Recordings == "" {
		conference.Recordings = "[]"
	}
	if conference.Settings == "" {
		conference.Settings = "{}"
	}

	return s.confRepo.Create(ctx, conference)
}

// GetByID is the public read path. Sprint 2.3 threaded accountID
// through from the handler; pass 0 from internal callers.
func (s *ConferenceService) GetByID(ctx context.Context, id, accountID uint) (*models.Conference, error) {
	conference, err := s.confRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("conference not found")
	}
	return conference, nil
}

func (s *ConferenceService) Update(ctx context.Context, conference *models.Conference) error {
	_, err := s.confRepo.FindByID(ctx, conference.ID, 0)
	if err != nil {
		return errors.New("conference not found")
	}

	return s.confRepo.Update(ctx, conference)
}

func (s *ConferenceService) Delete(ctx context.Context, id uint) error {
	return s.confRepo.Delete(ctx, id)
}

func (s *ConferenceService) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conference], error) {
	return s.confRepo.ListByContext(ctx, contextType, contextID, 0, params)
}

// Conference lifecycle

func (s *ConferenceService) StartConference(ctx context.Context, id uint) (*models.Conference, error) {
	conference, err := s.confRepo.FindByID(ctx, id, 0)
	if err != nil {
		return nil, errors.New("conference not found")
	}

	if conference.StartedAt != nil && conference.EndedAt == nil {
		return nil, errors.New("conference is already in progress")
	}

	now := time.Now()
	conference.StartedAt = &now
	conference.EndedAt = nil
	conference.JoinURL = fmt.Sprintf("/conferences/%d/join", conference.ID)
	conference.WorkflowState = "active"

	if err := s.confRepo.Update(ctx, conference); err != nil {
		return nil, err
	}

	return conference, nil
}

func (s *ConferenceService) EndConference(ctx context.Context, id uint) (*models.Conference, error) {
	conference, err := s.confRepo.FindByID(ctx, id, 0)
	if err != nil {
		return nil, errors.New("conference not found")
	}

	if conference.StartedAt == nil {
		return nil, errors.New("conference has not been started")
	}

	if conference.EndedAt != nil {
		return nil, errors.New("conference has already ended")
	}

	now := time.Now()
	conference.EndedAt = &now
	conference.WorkflowState = "ended"

	if err := s.confRepo.Update(ctx, conference); err != nil {
		return nil, err
	}

	return conference, nil
}

func (s *ConferenceService) JoinConference(ctx context.Context, conferenceID, userID uint) (string, error) {
	conference, err := s.confRepo.FindByID(ctx, conferenceID, 0)
	if err != nil {
		return "", errors.New("conference not found")
	}

	if conference.StartedAt == nil {
		return "", errors.New("conference has not been started")
	}

	if conference.EndedAt != nil {
		return "", errors.New("conference has already ended")
	}

	// Generate a placeholder join URL for the user
	joinURL := fmt.Sprintf("/conferences/%d/join?user_id=%d&token=placeholder", conferenceID, userID)

	return joinURL, nil
}

func (s *ConferenceService) GetRecordings(ctx context.Context, id uint) (string, error) {
	conference, err := s.confRepo.FindByID(ctx, id, 0)
	if err != nil {
		return "", errors.New("conference not found")
	}

	return conference.Recordings, nil
}

// Participant management

func (s *ConferenceService) AddParticipant(ctx context.Context, conferenceID, userID uint, participationType string) error {
	// Check conference exists
	_, err := s.confRepo.FindByID(ctx, conferenceID, 0)
	if err != nil {
		return errors.New("conference not found")
	}

	if participationType == "" {
		participationType = "invitee"
	}
	if participationType != "initiator" && participationType != "invitee" && participationType != "observer" {
		return errors.New("participation_type must be 'initiator', 'invitee', or 'observer'")
	}

	// Check if participant already exists
	existing, _ := s.participantRepo.FindByConferenceAndUser(ctx, conferenceID, userID, 0)
	if existing != nil {
		return errors.New("user is already a participant in this conference")
	}

	participant := &models.ConferenceParticipant{
		ConferenceID:      conferenceID,
		UserID:            userID,
		ParticipationType: participationType,
	}

	return s.participantRepo.Create(ctx, participant)
}

func (s *ConferenceService) RemoveParticipant(ctx context.Context, conferenceID, userID uint) error {
	participant, err := s.participantRepo.FindByConferenceAndUser(ctx, conferenceID, userID, 0)
	if err != nil {
		return errors.New("participant not found")
	}

	return s.participantRepo.Delete(ctx, participant.ID)
}

func (s *ConferenceService) ListParticipants(ctx context.Context, conferenceID uint) ([]models.ConferenceParticipant, error) {
	return s.participantRepo.ListByConferenceID(ctx, conferenceID, 0)
}
