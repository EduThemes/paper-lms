package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------------------------------------------------------------- exports

type contentExportRepo struct {
	db *gorm.DB
}

func NewContentExportRepository(db *gorm.DB) repository.ContentExportRepository {
	return &contentExportRepo{db: db}
}

func (r *contentExportRepo) Create(ctx context.Context, export *models.ContentExport) error {
	return r.db.WithContext(ctx).Create(export).Error
}

func (r *contentExportRepo) FindByID(ctx context.Context, id, accountID uint) (*models.ContentExport, error) {
	var export models.ContentExport
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.First(&export, id).Error; err != nil {
		return nil, err
	}
	return &export, nil
}

func (r *contentExportRepo) Update(ctx context.Context, export *models.ContentExport) error {
	return r.db.WithContext(ctx).Save(export).Error
}

func (r *contentExportRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentExport], error) {
	var items []models.ContentExport
	var count int64
	q := r.db.WithContext(ctx).Model(&models.ContentExport{}).Where("course_id = ?", courseID)
	if err := q.Count(&count).Error; err != nil {
		return nil, err
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 20
	}
	if err := q.Order("created_at DESC, id DESC").
		Offset((params.Page - 1) * params.PerPage).Limit(params.PerPage).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.ContentExport]{
		Items: items, TotalCount: count, Page: params.Page, PerPage: params.PerPage,
	}, nil
}

func (r *contentExportRepo) Claim(ctx context.Context, id uint) (bool, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&models.ContentExport{}).
		Where("id = ? AND workflow_state = ?", id, models.ContentExportCreated).
		Updates(map[string]interface{}{
			"workflow_state": models.ContentExportExporting,
			"started_at":     now,
			"updated_at":     now,
		})
	return res.RowsAffected == 1, res.Error
}

func (r *contentExportRepo) ListPendingIDs(ctx context.Context, olderThan time.Time, limit int) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.ContentExport{}).
		Where("workflow_state = ? AND created_at < ?", models.ContentExportCreated, olderThan).
		Order("id").Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

func (r *contentExportRepo) FailStale(ctx context.Context, before time.Time) (int64, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&models.ContentExport{}).
		Where("workflow_state = ? AND started_at < ?", models.ContentExportExporting, before).
		Updates(map[string]interface{}{
			"workflow_state": models.ContentExportFailed,
			"error_message":  "export was interrupted; please start it again",
			"finished_at":    now,
			"updated_at":     now,
		})
	return res.RowsAffected, res.Error
}

// ------------------------------------------------------- migration queue

type contentMigrationQueue struct {
	db *gorm.DB
}

func NewContentMigrationQueue(db *gorm.DB) repository.ContentMigrationQueue {
	return &contentMigrationQueue{db: db}
}

func (q *contentMigrationQueue) ClaimMigration(ctx context.Context, id uint) (bool, error) {
	now := time.Now()
	res := q.db.WithContext(ctx).Model(&models.ContentMigration{}).
		Where("id = ? AND workflow_state = ?", id, models.ContentMigrationQueued).
		Updates(map[string]interface{}{
			"workflow_state": models.ContentMigrationRunning,
			"started_at":     now,
			"updated_at":     now,
		})
	return res.RowsAffected == 1, res.Error
}

func (q *contentMigrationQueue) ListQueuedMigrationIDs(ctx context.Context, olderThan time.Time, limit int) ([]uint, error) {
	var ids []uint
	err := q.db.WithContext(ctx).Model(&models.ContentMigration{}).
		Where("workflow_state = ? AND created_at < ?", models.ContentMigrationQueued, olderThan).
		Order("id").Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

func (q *contentMigrationQueue) FailStaleMigrations(ctx context.Context, before time.Time) (int64, error) {
	now := time.Now()
	res := q.db.WithContext(ctx).Model(&models.ContentMigration{}).
		Where("workflow_state = ? AND started_at < ?", models.ContentMigrationRunning, before).
		Updates(map[string]interface{}{
			"workflow_state": models.ContentMigrationFailed,
			"error_message":  "migration was interrupted; please start it again",
			"finished_at":    now,
			"updated_at":     now,
		})
	return res.RowsAffected, res.Error
}

// --------------------------------------------------------- blueprint locks

type blueprintContentLockRepo struct {
	db *gorm.DB
}

func NewBlueprintContentLockRepository(db *gorm.DB) repository.BlueprintContentLockRepository {
	return &blueprintContentLockRepo{db: db}
}

func (r *blueprintContentLockRepo) Find(ctx context.Context, templateID uint, contentType string, contentID uint) (*models.BlueprintContentLock, error) {
	var lock models.BlueprintContentLock
	err := r.db.WithContext(ctx).
		Where("blueprint_template_id = ? AND content_type = ? AND content_id = ?", templateID, contentType, contentID).
		First(&lock).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &lock, nil
}

func (r *blueprintContentLockRepo) ListByTemplate(ctx context.Context, templateID uint) ([]models.BlueprintContentLock, error) {
	var locks []models.BlueprintContentLock
	err := r.db.WithContext(ctx).Where("blueprint_template_id = ?", templateID).Order("id").Find(&locks).Error
	return locks, err
}

func (r *blueprintContentLockRepo) Upsert(ctx context.Context, lock *models.BlueprintContentLock) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "blueprint_template_id"}, {Name: "content_type"}, {Name: "content_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"restrictions", "updated_at"}),
		}).
		Create(lock).Error
}

func (r *blueprintContentLockRepo) Delete(ctx context.Context, templateID uint, contentType string, contentID uint) error {
	return r.db.WithContext(ctx).
		Where("blueprint_template_id = ? AND content_type = ? AND content_id = ?", templateID, contentType, contentID).
		Delete(&models.BlueprintContentLock{}).Error
}
