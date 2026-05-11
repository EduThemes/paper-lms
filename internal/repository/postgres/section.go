package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type sectionRepo struct {
	db *gorm.DB
}

func NewSectionRepository(db *gorm.DB) repository.SectionRepository {
	return &sectionRepo{db: db}
}

func (r *sectionRepo) Create(ctx context.Context, section *models.CourseSection) error {
	return r.db.WithContext(ctx).Create(section).Error
}

func (r *sectionRepo) FindByID(ctx context.Context, id uint) (*models.CourseSection, error) {
	var section models.CourseSection
	if err := r.db.WithContext(ctx).First(&section, id).Error; err != nil {
		return nil, err
	}
	return &section, nil
}

func (r *sectionRepo) FindBySISSectionID(ctx context.Context, sisSectionID string) (*models.CourseSection, error) {
	var section models.CourseSection
	if err := r.db.WithContext(ctx).Where("sis_section_id = ?", sisSectionID).First(&section).Error; err != nil {
		return nil, err
	}
	return &section, nil
}

func (r *sectionRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CourseSection], error) {
	var sections []models.CourseSection
	var count int64

	query := r.db.WithContext(ctx).Model(&models.CourseSection{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&sections).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.CourseSection]{
		Items:      sections,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
