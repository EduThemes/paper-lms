package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// StudentAccommodationRepository defines the interface for student accommodation persistence.
type StudentAccommodationRepository interface {
	Create(ctx context.Context, accommodation *models.StudentAccommodation) error
	FindByID(ctx context.Context, id uint) (*models.StudentAccommodation, error)
	Update(ctx context.Context, accommodation *models.StudentAccommodation) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.StudentAccommodation], error)
	ListByUserAndCourse(ctx context.Context, userID uint, courseID uint) ([]models.StudentAccommodation, error)
	ListActiveByUserID(ctx context.Context, userID uint) ([]models.StudentAccommodation, error)
}

type studentAccommodationRepo struct {
	db *gorm.DB
}

func NewStudentAccommodationRepository(db *gorm.DB) StudentAccommodationRepository {
	return &studentAccommodationRepo{db: db}
}

func (r *studentAccommodationRepo) Create(ctx context.Context, accommodation *models.StudentAccommodation) error {
	return r.db.WithContext(ctx).Create(accommodation).Error
}

func (r *studentAccommodationRepo) FindByID(ctx context.Context, id uint) (*models.StudentAccommodation, error) {
	var accommodation models.StudentAccommodation
	if err := r.db.WithContext(ctx).First(&accommodation, id).Error; err != nil {
		return nil, err
	}
	return &accommodation, nil
}

func (r *studentAccommodationRepo) Update(ctx context.Context, accommodation *models.StudentAccommodation) error {
	return r.db.WithContext(ctx).Save(accommodation).Error
}

func (r *studentAccommodationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.StudentAccommodation{}).Where("id = ?", id).Update("status", "inactive").Error
}

func (r *studentAccommodationRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.StudentAccommodation], error) {
	var items []models.StudentAccommodation
	var totalCount int64

	query := r.db.WithContext(ctx).Model(&models.StudentAccommodation{}).Where("user_id = ? AND status != ?", userID, "inactive")
	query.Count(&totalCount)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.StudentAccommodation]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *studentAccommodationRepo) ListByUserAndCourse(ctx context.Context, userID uint, courseID uint) ([]models.StudentAccommodation, error) {
	var items []models.StudentAccommodation
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND (course_id = ? OR course_id IS NULL) AND status = ?", userID, courseID, "active").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *studentAccommodationRepo) ListActiveByUserID(ctx context.Context, userID uint) ([]models.StudentAccommodation, error) {
	var items []models.StudentAccommodation
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND effective_from <= ? AND (effective_until IS NULL OR effective_until >= ?)", userID, "active", now, now).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// AccommodationApplicationRepository defines the interface for accommodation application tracking.
type AccommodationApplicationRepository interface {
	Create(ctx context.Context, application *models.AccommodationApplication) error
	FindByResourceAndUser(ctx context.Context, resourceType string, resourceID uint, userID uint) (*models.AccommodationApplication, error)
	ListByAccommodationID(ctx context.Context, accommodationID uint) ([]models.AccommodationApplication, error)
}

type accommodationApplicationRepo struct {
	db *gorm.DB
}

func NewAccommodationApplicationRepository(db *gorm.DB) AccommodationApplicationRepository {
	return &accommodationApplicationRepo{db: db}
}

func (r *accommodationApplicationRepo) Create(ctx context.Context, application *models.AccommodationApplication) error {
	return r.db.WithContext(ctx).Create(application).Error
}

func (r *accommodationApplicationRepo) FindByResourceAndUser(ctx context.Context, resourceType string, resourceID uint, userID uint) (*models.AccommodationApplication, error) {
	var application models.AccommodationApplication
	if err := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ? AND user_id = ?", resourceType, resourceID, userID).
		Order("applied_at DESC").
		First(&application).Error; err != nil {
		return nil, err
	}
	return &application, nil
}

func (r *accommodationApplicationRepo) ListByAccommodationID(ctx context.Context, accommodationID uint) ([]models.AccommodationApplication, error) {
	var items []models.AccommodationApplication
	if err := r.db.WithContext(ctx).
		Where("accommodation_id = ?", accommodationID).
		Order("applied_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
