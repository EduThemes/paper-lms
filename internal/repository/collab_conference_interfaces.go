package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Collaborations

type CollaborationRepository interface {
	Create(ctx context.Context, collaboration *models.Collaboration) error
	FindByID(ctx context.Context, id uint) (*models.Collaboration, error)
	Update(ctx context.Context, collaboration *models.Collaboration) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.Collaboration], error)
}

// Conferences

type ConferenceRepository interface {
	Create(ctx context.Context, conference *models.Conference) error
	FindByID(ctx context.Context, id uint) (*models.Conference, error)
	Update(ctx context.Context, conference *models.Conference) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.Conference], error)
}

type ConferenceParticipantRepository interface {
	Create(ctx context.Context, participant *models.ConferenceParticipant) error
	FindByID(ctx context.Context, id uint) (*models.ConferenceParticipant, error)
	Delete(ctx context.Context, id uint) error
	ListByConferenceID(ctx context.Context, conferenceID uint) ([]models.ConferenceParticipant, error)
	FindByConferenceAndUser(ctx context.Context, conferenceID, userID uint) (*models.ConferenceParticipant, error)
}
