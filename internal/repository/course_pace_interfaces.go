package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Course Pacing

type CoursePaceRepository interface {
	Create(ctx context.Context, pace *models.CoursePace) error
	// FindByID — 13.1.D: tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.CoursePace, error)
	Update(ctx context.Context, pace *models.CoursePace) error
	Delete(ctx context.Context, id uint) error
	FindByCourseID(ctx context.Context, courseID uint) (*models.CoursePace, error)
	FindByUserID(ctx context.Context, courseID uint, userID uint) (*models.CoursePace, error)
	FindBySectionID(ctx context.Context, courseID uint, sectionID uint) (*models.CoursePace, error)
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.CoursePace], error)
}

type CoursePaceModuleItemRepository interface {
	Create(ctx context.Context, item *models.CoursePaceModuleItem) error
	// FindByID — 13.1.D: tenant scope via pace→course→account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.CoursePaceModuleItem, error)
	Update(ctx context.Context, item *models.CoursePaceModuleItem) error
	Delete(ctx context.Context, id uint) error
	ListByPaceID(ctx context.Context, paceID uint) ([]models.CoursePaceModuleItem, error)
	BulkUpsert(ctx context.Context, items []models.CoursePaceModuleItem) error
}
