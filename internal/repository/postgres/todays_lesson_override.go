package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type todaysLessonOverrideRepo struct {
	db *gorm.DB
}

func NewTodaysLessonOverrideRepository(db *gorm.DB) repository.TodaysLessonOverrideRepository {
	return &todaysLessonOverrideRepo{db: db}
}

func (r *todaysLessonOverrideRepo) Create(ctx context.Context, override *models.TodaysLessonOverride) error {
	return r.db.WithContext(ctx).Create(override).Error
}

func (r *todaysLessonOverrideRepo) FindByID(ctx context.Context, id uint) (*models.TodaysLessonOverride, error) {
	var override models.TodaysLessonOverride
	if err := r.db.WithContext(ctx).First(&override, id).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *todaysLessonOverrideRepo) Update(ctx context.Context, override *models.TodaysLessonOverride) error {
	return r.db.WithContext(ctx).Save(override).Error
}

func (r *todaysLessonOverrideRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.TodaysLessonOverride{}, id).Error
}

func (r *todaysLessonOverrideRepo) FindByCourseAndDate(ctx context.Context, courseID uint, date time.Time) (*models.TodaysLessonOverride, error) {
	var override models.TodaysLessonOverride
	if err := r.db.WithContext(ctx).Where("course_id = ? AND date = ?", courseID, date).First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *todaysLessonOverrideRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.TodaysLessonOverride], error) {
	var overrides []models.TodaysLessonOverride
	var count int64

	query := r.db.WithContext(ctx).Model(&models.TodaysLessonOverride{}).Where("course_id = ?", courseID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("date DESC").Find(&overrides).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.TodaysLessonOverride]{
		Items:      overrides,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
