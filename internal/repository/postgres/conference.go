package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type conferenceRepo struct {
	db *gorm.DB
}

func NewConferenceRepository(db *gorm.DB) repository.ConferenceRepository {
	return &conferenceRepo{db: db}
}

func (r *conferenceRepo) Create(ctx context.Context, conference *models.Conference) error {
	return r.db.WithContext(ctx).Create(conference).Error
}

func (r *conferenceRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Conference, error) {
	var conference models.Conference
	q := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted")
	if accountID != 0 {
		// Polymorphic context_type branching. Unknown types deny the
		// read (we list only the known-safe types). See 13.1.E
		// principle: an unknown context_type means we can't prove
		// tenant ownership, so deny.
		q = q.Where(`
			(context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			OR (context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Group' AND context_id IN (SELECT id FROM groups WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))
		`, accountID, accountID, accountID)
	}
	if err := q.First(&conference, id).Error; err != nil {
		return nil, err
	}
	return &conference, nil
}

func (r *conferenceRepo) Update(ctx context.Context, conference *models.Conference) error {
	return r.db.WithContext(ctx).Save(conference).Error
}

func (r *conferenceRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Conference{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *conferenceRepo) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conference], error) {
	var conferences []models.Conference
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Conference{}).
		Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")

	if accountID != 0 {
		switch contextType {
		case "Course":
			query = query.Where("context_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
		case "Account":
			query = query.Where("context_id = ?", accountID)
		case "Group":
			query = query.Where("context_id IN (SELECT id FROM groups WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?))", accountID)
		default:
			// Unknown context_type → deny by forcing an empty result.
			query = query.Where("1 = 0")
		}
	}

	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&conferences).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Conference]{
		Items:      conferences,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
