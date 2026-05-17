package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type ContentMigrationRepository interface {
	Create(ctx context.Context, migration *models.ContentMigration) error
	FindByID(ctx context.Context, id uint) (*models.ContentMigration, error)
	Update(ctx context.Context, migration *models.ContentMigration) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.ContentMigration], error)
}

type BlueprintTemplateRepository interface {
	Create(ctx context.Context, template *models.BlueprintTemplate) error
	FindByID(ctx context.Context, id uint) (*models.BlueprintTemplate, error)
	FindByCourseID(ctx context.Context, courseID uint) (*models.BlueprintTemplate, error)
	Update(ctx context.Context, template *models.BlueprintTemplate) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.BlueprintTemplate], error)
}

type BlueprintSubscriptionRepository interface {
	Create(ctx context.Context, subscription *models.BlueprintSubscription) error
	FindByID(ctx context.Context, id uint) (*models.BlueprintSubscription, error)
	FindByTemplateAndChild(ctx context.Context, templateID, childCourseID uint) (*models.BlueprintSubscription, error)
	Update(ctx context.Context, subscription *models.BlueprintSubscription) error
	Delete(ctx context.Context, id uint) error
	ListByTemplateID(ctx context.Context, templateID uint, params PaginationParams) (*PaginatedResult[models.BlueprintSubscription], error)
	ListByChildCourseID(ctx context.Context, childCourseID uint, params PaginationParams) (*PaginatedResult[models.BlueprintSubscription], error)
}

type BlueprintMigrationRepository interface {
	Create(ctx context.Context, migration *models.BlueprintMigration) error
	FindByID(ctx context.Context, id uint) (*models.BlueprintMigration, error)
	Update(ctx context.Context, migration *models.BlueprintMigration) error
	Delete(ctx context.Context, id uint) error
	ListByTemplateID(ctx context.Context, templateID uint, params PaginationParams) (*PaginatedResult[models.BlueprintMigration], error)
	ListBySubscriptionID(ctx context.Context, subscriptionID uint, params PaginationParams) (*PaginatedResult[models.BlueprintMigration], error)
}

type OneRosterConnectionRepository interface {
	Create(ctx context.Context, conn *models.OneRosterConnection) error
	// 13.1.D — direct account_id column.
	FindByID(ctx context.Context, id, accountID uint) (*models.OneRosterConnection, error)
	Update(ctx context.Context, conn *models.OneRosterConnection) error
	Delete(ctx context.Context, id uint) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.OneRosterConnection], error)
	FindByAccountAndName(ctx context.Context, accountID uint, name string) (*models.OneRosterConnection, error)
	ListAutoSync(ctx context.Context) ([]models.OneRosterConnection, error)
}

type OneRosterSyncLogRepository interface {
	Create(ctx context.Context, log *models.OneRosterSyncLog) error
	Update(ctx context.Context, log *models.OneRosterSyncLog) error
	ListByConnectionID(ctx context.Context, connectionID uint, params PaginationParams) (*PaginatedResult[models.OneRosterSyncLog], error)
	GetLatestByConnectionID(ctx context.Context, connectionID uint) (*models.OneRosterSyncLog, error)
}
