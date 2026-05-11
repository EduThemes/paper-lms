package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type conferenceParticipantRepo struct {
	db *gorm.DB
}

func NewConferenceParticipantRepository(db *gorm.DB) repository.ConferenceParticipantRepository {
	return &conferenceParticipantRepo{db: db}
}

func (r *conferenceParticipantRepo) Create(ctx context.Context, participant *models.ConferenceParticipant) error {
	return r.db.WithContext(ctx).Create(participant).Error
}

func (r *conferenceParticipantRepo) FindByID(ctx context.Context, id uint) (*models.ConferenceParticipant, error) {
	var participant models.ConferenceParticipant
	if err := r.db.WithContext(ctx).Preload("User").First(&participant, id).Error; err != nil {
		return nil, err
	}
	return &participant, nil
}

func (r *conferenceParticipantRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ConferenceParticipant{}, id).Error
}

func (r *conferenceParticipantRepo) ListByConferenceID(ctx context.Context, conferenceID uint) ([]models.ConferenceParticipant, error) {
	var participants []models.ConferenceParticipant
	if err := r.db.WithContext(ctx).Preload("User").Where("conference_id = ?", conferenceID).Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *conferenceParticipantRepo) FindByConferenceAndUser(ctx context.Context, conferenceID, userID uint) (*models.ConferenceParticipant, error) {
	var participant models.ConferenceParticipant
	if err := r.db.WithContext(ctx).Preload("User").Where("conference_id = ? AND user_id = ?", conferenceID, userID).First(&participant).Error; err != nil {
		return nil, err
	}
	return &participant, nil
}
