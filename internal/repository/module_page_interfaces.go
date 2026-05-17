package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type ModuleRepository interface {
	Create(ctx context.Context, module *models.ContextModule) error
	// 13.1.D — accountID scopes the read to a single tenant via the
	// parent course's account_id. 0 means "no tenant scope" (internal
	// callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.ContextModule, error)
	Update(ctx context.Context, module *models.ContextModule) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.ContextModule], error)
	FindActiveByDateRange(ctx context.Context, courseID uint, date time.Time) (*models.ContextModule, error)
	ReorderModules(ctx context.Context, courseID uint, moduleIDs []uint) error
}

type ModuleItemRepository interface {
	Create(ctx context.Context, item *models.ContentTag) error
	FindByID(ctx context.Context, id uint) (*models.ContentTag, error)
	Update(ctx context.Context, item *models.ContentTag) error
	Delete(ctx context.Context, id uint) error
	ListByModuleID(ctx context.Context, moduleID uint, params PaginationParams) (*PaginatedResult[models.ContentTag], error)
	ReorderItems(ctx context.Context, moduleID uint, itemIDs []uint) error
	MoveItemToModule(ctx context.Context, itemID uint, targetModuleID uint, position int) error
}

type PageRepository interface {
	Create(ctx context.Context, page *models.WikiPage) error
	// 13.1.D — tenant scope via parent course's account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.WikiPage, error)
	FindByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error)
	FindPublicByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error)
	Update(ctx context.Context, page *models.WikiPage) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.WikiPage], error)
}

type ModulePrerequisiteRepository interface {
	SetPrerequisites(ctx context.Context, moduleID uint, prerequisiteModuleIDs []uint) error
	GetPrerequisites(ctx context.Context, moduleID uint) ([]uint, error)
	GetModulesWithPrerequisite(ctx context.Context, prerequisiteModuleID uint) ([]uint, error)
}

type WikiPageRevisionRepository interface {
	Create(ctx context.Context, revision *models.WikiPageRevision) error
	FindByID(ctx context.Context, id uint) (*models.WikiPageRevision, error)
	ListByPageID(ctx context.Context, pageID uint, params PaginationParams) (*PaginatedResult[models.WikiPageRevision], error)
	GetLatestRevision(ctx context.Context, pageID uint) (*models.WikiPageRevision, error)
	GetRevisionByNumber(ctx context.Context, pageID uint, revisionNumber int) (*models.WikiPageRevision, error)
}
