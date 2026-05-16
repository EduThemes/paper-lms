package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type coursePaceRepo struct {
	db *gorm.DB
}

func NewCoursePaceRepository(db *gorm.DB) repository.CoursePaceRepository {
	return &coursePaceRepo{db: db}
}

func (r *coursePaceRepo) Create(ctx context.Context, pace *models.CoursePace) error {
	return r.db.WithContext(ctx).Create(pace).Error
}

func (r *coursePaceRepo) FindByID(ctx context.Context, id, accountID uint) (*models.CoursePace, error) {
	var pace models.CoursePace
	q := r.db.WithContext(ctx).Where("id = ? AND workflow_state != ?", id, "deleted")
	if accountID != 0 {
		// Scope through parent course's account_id.
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.First(&pace).Error; err != nil {
		return nil, err
	}
	return &pace, nil
}

func (r *coursePaceRepo) Update(ctx context.Context, pace *models.CoursePace) error {
	return r.db.WithContext(ctx).Save(pace).Error
}

func (r *coursePaceRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.CoursePace{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *coursePaceRepo) FindByCourseID(ctx context.Context, courseID uint) (*models.CoursePace, error) {
	var pace models.CoursePace
	if err := r.db.WithContext(ctx).
		Where("course_id = ? AND user_id IS NULL AND course_section_id IS NULL AND workflow_state != ?", courseID, "deleted").
		First(&pace).Error; err != nil {
		return nil, err
	}
	return &pace, nil
}

func (r *coursePaceRepo) FindByUserID(ctx context.Context, courseID uint, userID uint) (*models.CoursePace, error) {
	var pace models.CoursePace
	if err := r.db.WithContext(ctx).
		Where("course_id = ? AND user_id = ? AND workflow_state != ?", courseID, userID, "deleted").
		First(&pace).Error; err != nil {
		return nil, err
	}
	return &pace, nil
}

func (r *coursePaceRepo) FindBySectionID(ctx context.Context, courseID uint, sectionID uint) (*models.CoursePace, error) {
	var pace models.CoursePace
	if err := r.db.WithContext(ctx).
		Where("course_id = ? AND course_section_id = ? AND workflow_state != ?", courseID, sectionID, "deleted").
		First(&pace).Error; err != nil {
		return nil, err
	}
	return &pace, nil
}

func (r *coursePaceRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CoursePace], error) {
	var paces []models.CoursePace
	var count int64

	query := r.db.WithContext(ctx).Model(&models.CoursePace{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&paces).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.CoursePace]{
		Items:      paces,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
