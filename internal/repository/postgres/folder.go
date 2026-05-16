package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type folderRepo struct {
	db *gorm.DB
}

func NewFolderRepository(db *gorm.DB) repository.FolderRepository {
	return &folderRepo{db: db}
}

func (r *folderRepo) Create(ctx context.Context, folder *models.Folder) error {
	return r.db.WithContext(ctx).Create(folder).Error
}

func (r *folderRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Folder, error) {
	var folder models.Folder
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Folders are polymorphic on (context_type, context_id). Branch the
		// tenant filter on context_type.
		q = q.Where(`
			(context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			OR (context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Group' AND context_id IN (
				SELECT g.id FROM groups g
				WHERE (g.context_type = 'Course' AND g.context_id IN (SELECT id FROM courses WHERE account_id = ?))
				   OR (g.context_type = 'Account' AND g.context_id = ?)
			))
			OR (context_type = 'User' AND context_id IN (SELECT id FROM users WHERE account_id = ?))
		`, accountID, accountID, accountID, accountID, accountID)
	}
	if err := q.First(&folder, id).Error; err != nil {
		return nil, err
	}
	return &folder, nil
}

func (r *folderRepo) Update(ctx context.Context, folder *models.Folder) error {
	return r.db.WithContext(ctx).Save(folder).Error
}

func (r *folderRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Folder{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *folderRepo) ListByContext(ctx context.Context, contextType string, contextID uint, parentFolderID *uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Folder], error) {
	var folders []models.Folder
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Folder{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	if parentFolderID != nil {
		query = query.Where("parent_folder_id = ?", *parentFolderID)
	}
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("position ASC, name ASC").Find(&folders).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Folder]{
		Items:      folders,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *folderRepo) FindRootFolder(ctx context.Context, contextType string, contextID uint) (*models.Folder, error) {
	var folder models.Folder
	if err := r.db.WithContext(ctx).Where("context_type = ? AND context_id = ? AND parent_folder_id IS NULL AND workflow_state != ?", contextType, contextID, "deleted").First(&folder).Error; err != nil {
		return nil, err
	}
	return &folder, nil
}
