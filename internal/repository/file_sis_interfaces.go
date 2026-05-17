package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type FolderRepository interface {
	Create(ctx context.Context, folder *models.Folder) error
	// 13.1.D — tenant scope via polymorphic context_type/context_id.
	// accountID==0 means "no scope" (background jobs, IMSCC import).
	FindByID(ctx context.Context, id, accountID uint) (*models.Folder, error)
	Update(ctx context.Context, folder *models.Folder) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, parentFolderID *uint, params PaginationParams) (*PaginatedResult[models.Folder], error)
	FindRootFolder(ctx context.Context, contextType string, contextID uint) (*models.Folder, error)
}

type AttachmentRepository interface {
	Create(ctx context.Context, attachment *models.Attachment) error
	// 13.1.D — tenant scope via parent folder's context (inherit-via-parent).
	FindByID(ctx context.Context, id, accountID uint) (*models.Attachment, error)
	Update(ctx context.Context, attachment *models.Attachment) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.Attachment], error)
	ListByFolderID(ctx context.Context, folderID uint, params PaginationParams) (*PaginatedResult[models.Attachment], error)
}

type SISBatchRepository interface {
	Create(ctx context.Context, batch *models.SISBatch) error
	FindByID(ctx context.Context, id uint) (*models.SISBatch, error)
	Update(ctx context.Context, batch *models.SISBatch) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.SISBatch], error)
}

type SISBatchErrorRepository interface {
	Create(ctx context.Context, batchError *models.SISBatchError) error
	ListByBatchID(ctx context.Context, batchID uint) ([]models.SISBatchError, error)
}
