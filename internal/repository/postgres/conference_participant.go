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

func (r *conferenceParticipantRepo) FindByID(ctx context.Context, id, accountID uint) (*models.ConferenceParticipant, error) {
	var participant models.ConferenceParticipant
	q := r.db.WithContext(ctx).Preload("User")
	if accountID != 0 {
		q = q.Where(conferenceTenantSubquery("conference_id"), accountID, accountID, accountID)
	}
	if err := q.First(&participant, id).Error; err != nil {
		return nil, err
	}
	return &participant, nil
}

// conferenceTenantSubquery builds a SQL fragment that filters rows
// whose conferenceFK is owned by `accountID` via the conference's
// polymorphic context_type. Unknown context_types are excluded.
// Caller binds 3 placeholders (Course, Account, Group accountID).
func conferenceTenantSubquery(conferenceColumn string) string {
	return conferenceColumn + ` IN (
		SELECT id FROM conferences WHERE
			(context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			OR (context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Group' AND context_id IN (SELECT id FROM groups WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))
	)`
}

func (r *conferenceParticipantRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ConferenceParticipant{}, id).Error
}

func (r *conferenceParticipantRepo) ListByConferenceID(ctx context.Context, conferenceID, accountID uint) ([]models.ConferenceParticipant, error) {
	var participants []models.ConferenceParticipant
	q := r.db.WithContext(ctx).Preload("User").Where("conference_id = ?", conferenceID)
	if accountID != 0 {
		q = q.Where(conferenceTenantSubquery("conference_id"), accountID, accountID, accountID)
	}
	if err := q.Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *conferenceParticipantRepo) FindByConferenceAndUser(ctx context.Context, conferenceID, userID, accountID uint) (*models.ConferenceParticipant, error) {
	var participant models.ConferenceParticipant
	q := r.db.WithContext(ctx).Preload("User").Where("conference_id = ? AND user_id = ?", conferenceID, userID)
	if accountID != 0 {
		q = q.Where(conferenceTenantSubquery("conference_id"), accountID, accountID, accountID)
	}
	if err := q.First(&participant).Error; err != nil {
		return nil, err
	}
	return &participant, nil
}
