package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type assignmentGroupRepo struct {
	db *gorm.DB
}

func NewAssignmentGroupRepository(db *gorm.DB) repository.AssignmentGroupRepository {
	return &assignmentGroupRepo{db: db}
}

func (r *assignmentGroupRepo) Create(ctx context.Context, group *models.AssignmentGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *assignmentGroupRepo) FindByID(ctx context.Context, id, accountID uint) (*models.AssignmentGroup, error) {
	var group models.AssignmentGroup
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Scope through the parent course's account_id. Mirrors the
		// child-table pattern from module.go / page.go (13.1.D).
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *assignmentGroupRepo) Update(ctx context.Context, group *models.AssignmentGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *assignmentGroupRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.AssignmentGroup{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *assignmentGroupRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AssignmentGroup], error) {
	var groups []models.AssignmentGroup
	var count int64

	query := r.db.WithContext(ctx).Model(&models.AssignmentGroup{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("position ASC, id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AssignmentGroup]{
		Items:      groups,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
