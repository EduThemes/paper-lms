package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type courseRepo struct {
	db *gorm.DB
}

func NewCourseRepository(db *gorm.DB) repository.CourseRepository {
	return &courseRepo{db: db}
}

func (r *courseRepo) Create(ctx context.Context, course *models.Course) error {
	return r.db.WithContext(ctx).Create(course).Error
}

func (r *courseRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Course, error) {
	var course models.Course
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&course, id).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *courseRepo) FindBySISCourseID(ctx context.Context, sisCourseID string) (*models.Course, error) {
	var course models.Course
	if err := r.db.WithContext(ctx).Where("sis_course_id = ?", sisCourseID).First(&course).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *courseRepo) Update(ctx context.Context, course *models.Course) error {
	return r.db.WithContext(ctx).Save(course).Error
}

func (r *courseRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Course{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *courseRepo) List(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	var courses []models.Course
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Course{}).Where("workflow_state != ?", "deleted")
	if accountID != 0 {
		query = query.Where("account_id = ?", accountID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&courses).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Course]{
		Items:      courses,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *courseRepo) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	var courses []models.Course
	var count int64

	subQuery := r.db.Model(&models.Enrollment{}).Select("course_id").Where("user_id = ? AND workflow_state = ?", userID, "active")

	query := r.db.WithContext(ctx).Model(&models.Course{}).Where("id IN (?) AND workflow_state != ?", subQuery, "deleted")
	if accountID != 0 {
		query = query.Where("account_id = ?", accountID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&courses).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Course]{
		Items:      courses,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
