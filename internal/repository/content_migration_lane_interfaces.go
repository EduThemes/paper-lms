package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ContentExportRepository persists course content exports.
type ContentExportRepository interface {
	Create(ctx context.Context, export *models.ContentExport) error
	// FindByID is tenant-scoped through the parent course's account_id.
	// accountID==0 disables the scope (the background worker only).
	FindByID(ctx context.Context, id, accountID uint) (*models.ContentExport, error)
	Update(ctx context.Context, export *models.ContentExport) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.ContentExport], error)
	// Claim atomically moves a created export to exporting; false means
	// another worker already took it (or it is not in created).
	Claim(ctx context.Context, id uint) (bool, error)
	// ListPendingIDs returns exports still in created, oldest first.
	ListPendingIDs(ctx context.Context, olderThan time.Time, limit int) ([]uint, error)
	// FailStale marks exports stuck in exporting since before `before`
	// as failed (a worker died mid-run).
	FailStale(ctx context.Context, before time.Time) (int64, error)
}

// ContentMigrationQueue is the worker-side view of content_migrations:
// claim a queued row, find queued rows nobody picked up, and fail rows
// left running by a dead worker. Kept apart from
// ContentMigrationRepository so its fakes stay small.
type ContentMigrationQueue interface {
	ClaimMigration(ctx context.Context, id uint) (bool, error)
	ListQueuedMigrationIDs(ctx context.Context, olderThan time.Time, limit int) ([]uint, error)
	FailStaleMigrations(ctx context.Context, before time.Time) (int64, error)
}

// BlueprintContentLockRepository stores per-item blueprint lock settings.
type BlueprintContentLockRepository interface {
	// Find returns the lock row for one blueprint item, or (nil, nil).
	Find(ctx context.Context, templateID uint, contentType string, contentID uint) (*models.BlueprintContentLock, error)
	ListByTemplate(ctx context.Context, templateID uint) ([]models.BlueprintContentLock, error)
	Upsert(ctx context.Context, lock *models.BlueprintContentLock) error
	Delete(ctx context.Context, templateID uint, contentType string, contentID uint) error
}
