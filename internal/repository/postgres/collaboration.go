package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type collaborationRepo struct {
	db *gorm.DB
}

func NewCollaborationRepository(db *gorm.DB) repository.CollaborationRepository {
	return &collaborationRepo{db: db}
}

func (r *collaborationRepo) Create(ctx context.Context, collaboration *models.Collaboration) error {
	return r.db.WithContext(ctx).Create(collaboration).Error
}

func (r *collaborationRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Collaboration, error) {
	var collaboration models.Collaboration
	q := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted")
	if accountID != 0 {
		// Polymorphic context_type branching. Unknown types deny the
		// read (only Course/Account/Group are tenant-resolvable).
		q = q.Where(`
			(context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			OR (context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Group' AND context_id IN (SELECT id FROM groups WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))
		`, accountID, accountID, accountID)
	}
	if err := q.First(&collaboration, id).Error; err != nil {
		return nil, err
	}
	return &collaboration, nil
}

func (r *collaborationRepo) Update(ctx context.Context, collaboration *models.Collaboration) error {
	return r.db.WithContext(ctx).Save(collaboration).Error
}

func (r *collaborationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Collaboration{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *collaborationRepo) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Collaboration], error) {
	var collaborations []models.Collaboration
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Collaboration{}).
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
			// Unknown context_type → deny.
			query = query.Where("1 = 0")
		}
	}

	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&collaborations).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Collaboration]{
		Items:      collaborations,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
