package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Collaborations

type CollaborationRepository interface {
	Create(ctx context.Context, collaboration *models.Collaboration) error
	// FindByID — 13.1.D: tenant-scoped via polymorphic context_type
	// branching (Course → parent JOIN, Account → direct, Group →
	// group→course JOIN). Unknown context_type denies the read
	// (returns gorm.ErrRecordNotFound) to avoid leaking IDs whose
	// ownership we can't prove. accountID==0 means "no scope"
	// (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.Collaboration, error)
	Update(ctx context.Context, collaboration *models.Collaboration) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.Collaboration], error)
}

// Conferences

type ConferenceRepository interface {
	Create(ctx context.Context, conference *models.Conference) error
	// FindByID — 13.1.D: tenant-scoped via polymorphic context_type
	// branching (Course → parent JOIN, Account → direct, Group →
	// group→course JOIN). Unknown context_type denies the read.
	// accountID==0 means "no scope" (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.Conference, error)
	Update(ctx context.Context, conference *models.Conference) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params PaginationParams) (*PaginatedResult[models.Conference], error)
}

type ConferenceParticipantRepository interface {
	Create(ctx context.Context, participant *models.ConferenceParticipant) error
	// FindByID — 13.1.D: tenant-scoped via the participant → conference
	// → polymorphic-context chain. accountID==0 means "no scope".
	FindByID(ctx context.Context, id, accountID uint) (*models.ConferenceParticipant, error)
	Delete(ctx context.Context, id uint) error
	ListByConferenceID(ctx context.Context, conferenceID, accountID uint) ([]models.ConferenceParticipant, error)
	FindByConferenceAndUser(ctx context.Context, conferenceID, userID, accountID uint) (*models.ConferenceParticipant, error)
}
