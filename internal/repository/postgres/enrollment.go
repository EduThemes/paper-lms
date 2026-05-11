package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type enrollmentRepo struct {
	db *gorm.DB
}

func NewEnrollmentRepository(db *gorm.DB) repository.EnrollmentRepository {
	return &enrollmentRepo{db: db}
}

func (r *enrollmentRepo) Create(ctx context.Context, enrollment *models.Enrollment) error {
	return r.db.WithContext(ctx).Create(enrollment).Error
}

func (r *enrollmentRepo) FindByID(ctx context.Context, id uint) (*models.Enrollment, error) {
	var enrollment models.Enrollment
	if err := r.db.WithContext(ctx).Preload("User").First(&enrollment, id).Error; err != nil {
		return nil, err
	}
	return &enrollment, nil
}

func (r *enrollmentRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	var enrollments []models.Enrollment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Enrollment{}).Where("course_id = ? AND workflow_state != ?", courseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Preload("User").Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&enrollments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Enrollment]{
		Items:      enrollments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *enrollmentRepo) ListByUserID(ctx context.Context, userID uint) ([]models.Enrollment, error) {
	var enrollments []models.Enrollment
	if err := r.db.WithContext(ctx).Where("user_id = ? AND workflow_state = ?", userID, "active").Find(&enrollments).Error; err != nil {
		return nil, err
	}
	return enrollments, nil
}

func (r *enrollmentRepo) Update(ctx context.Context, enrollment *models.Enrollment) error {
	return r.db.WithContext(ctx).Save(enrollment).Error
}

func (r *enrollmentRepo) FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.Enrollment, error) {
	var enrollment models.Enrollment
	if err := r.db.WithContext(ctx).Where("user_id = ? AND course_id = ? AND workflow_state = ?", userID, courseID, "active").First(&enrollment).Error; err != nil {
		return nil, err
	}
	return &enrollment, nil
}

func (r *enrollmentRepo) CountByCourseIDs(ctx context.Context, courseIDs []uint) (map[uint]int64, error) {
	if len(courseIDs) == 0 {
		return map[uint]int64{}, nil
	}
	type result struct {
		CourseID uint
		Count    int64
	}
	var results []result
	err := r.db.WithContext(ctx).
		Model(&models.Enrollment{}).
		Select("course_id, count(*) as count").
		Where("course_id IN ? AND workflow_state = ? AND type = ?", courseIDs, "active", "StudentEnrollment").
		Group("course_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(results))
	for _, r := range results {
		counts[r.CourseID] = r.Count
	}
	return counts, nil
}
