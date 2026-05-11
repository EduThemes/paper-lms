package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type assignmentRepo struct {
	db *gorm.DB
}

func NewAssignmentRepository(db *gorm.DB) repository.AssignmentRepository {
	return &assignmentRepo{db: db}
}

func (r *assignmentRepo) Create(ctx context.Context, assignment *models.Assignment) error {
	return r.db.WithContext(ctx).Create(assignment).Error
}

func (r *assignmentRepo) FindByID(ctx context.Context, id uint) (*models.Assignment, error) {
	var assignment models.Assignment
	if err := r.db.WithContext(ctx).First(&assignment, id).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *assignmentRepo) FindByIDs(ctx context.Context, ids []uint) ([]models.Assignment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var assignments []models.Assignment
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *assignmentRepo) Update(ctx context.Context, assignment *models.Assignment) error {
	return r.db.WithContext(ctx).Save(assignment).Error
}

func (r *assignmentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Assignment{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *assignmentRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Assignment], error) {
	var assignments []models.Assignment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Assignment{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("position ASC, id ASC").Find(&assignments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Assignment]{
		Items:      assignments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
