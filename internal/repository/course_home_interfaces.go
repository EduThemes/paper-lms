package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type CourseHomeButtonRepository interface {
	Create(ctx context.Context, button *models.CourseHomeButton) error
	FindByID(ctx context.Context, id uint) (*models.CourseHomeButton, error)
	Update(ctx context.Context, button *models.CourseHomeButton) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint) ([]models.CourseHomeButton, error)
	BulkUpdatePositions(ctx context.Context, courseID uint, positions map[uint]int) error
}

type TodaysLessonOverrideRepository interface {
	Create(ctx context.Context, override *models.TodaysLessonOverride) error
	FindByID(ctx context.Context, id uint) (*models.TodaysLessonOverride, error)
	Update(ctx context.Context, override *models.TodaysLessonOverride) error
	Delete(ctx context.Context, id uint) error
	FindByCourseAndDate(ctx context.Context, courseID uint, date time.Time) (*models.TodaysLessonOverride, error)
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.TodaysLessonOverride], error)
}

type CourseVisitRepository interface {
	Upsert(ctx context.Context, visit *models.CourseVisit) error
	FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.CourseVisit, error)
}
