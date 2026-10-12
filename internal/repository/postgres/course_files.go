package postgres

import (
	"context"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type courseFilesRepo struct {
	db *gorm.DB
}

// NewCourseFilesRepository builds the file-tree repository (G070-G072).
func NewCourseFilesRepository(db *gorm.DB) repository.CourseFilesRepository {
	return &courseFilesRepo{db: db}
}

// blockedFoldersSQL selects the ids of folders whose contents a listing
// must skip: deleted folders for everyone, plus locked and hidden ones
// for learners, and every folder below those.
func blockedFoldersSQL(learner bool) string {
	seed := "workflow_state = 'deleted'"
	if learner {
		seed = "(workflow_state = 'deleted' OR locked OR hidden)"
	}
	return `WITH RECURSIVE blocked AS (
		SELECT id FROM folders WHERE context_type = ? AND context_id = ? AND ` + seed + `
		UNION
		SELECT f.id FROM folders f JOIN blocked b ON f.parent_folder_id = b.id
	) SELECT id FROM blocked`
}

func (r *courseFilesRepo) ListFiles(ctx context.Context, q repository.FileTreeQuery, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	query := r.db.WithContext(ctx).Model(&models.Attachment{}).
		Where("context_type = ? AND context_id = ? AND workflow_state <> ?", q.ContextType, q.ContextID, models.AttachmentDeleted).
		Where("(folder_id IS NULL OR folder_id NOT IN ("+blockedFoldersSQL(q.LearnerView)+"))", q.ContextType, q.ContextID)
	if q.LearnerView {
		query = query.Where("locked = ? AND hidden = ?", false, false)
	}
	switch q.Scope {
	case repository.FolderScopeRoot:
		if q.RootFolderID != 0 {
			query = query.Where("(folder_id IS NULL OR folder_id = ?)", q.RootFolderID)
		} else {
			query = query.Where("folder_id IS NULL")
		}
	case repository.FolderScopeFolder:
		query = query.Where("folder_id = ?", q.FolderID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}
	var files []models.Attachment
	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("display_name ASC, id ASC").Offset(offset).Limit(params.PerPage).Find(&files).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.Attachment]{Items: files, TotalCount: count, Page: params.Page, PerPage: params.PerPage}, nil
}

func (r *courseFilesRepo) ListFolders(ctx context.Context, q repository.FileTreeQuery, params repository.PaginationParams) (*repository.PaginatedResult[models.Folder], error) {
	query := r.db.WithContext(ctx).Model(&models.Folder{}).
		Where("context_type = ? AND context_id = ?", q.ContextType, q.ContextID).
		Where("id NOT IN ("+blockedFoldersSQL(q.LearnerView)+")", q.ContextType, q.ContextID)
	switch q.Scope {
	case repository.FolderScopeRoot:
		if q.RootFolderID != 0 {
			query = query.Where("parent_folder_id = ?", q.RootFolderID)
		} else {
			query = query.Where("1 = 0")
		}
	case repository.FolderScopeFolder:
		query = query.Where("parent_folder_id = ?", q.FolderID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}
	var folders []models.Folder
	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("position ASC, name ASC, id ASC").Offset(offset).Limit(params.PerPage).Find(&folders).Error; err != nil {
		return nil, err
	}
	return &repository.PaginatedResult[models.Folder]{Items: folders, TotalCount: count, Page: params.Page, PerPage: params.PerPage}, nil
}

func (r *courseFilesRepo) FolderChain(ctx context.Context, folderID uint) ([]models.Folder, error) {
	var chain []models.Folder
	err := r.db.WithContext(ctx).Raw(`
		WITH RECURSIVE chain AS (
			SELECT f.*, 0 AS depth FROM folders f WHERE f.id = ?
			UNION ALL
			SELECT p.*, c.depth + 1 FROM folders p JOIN chain c ON p.id = c.parent_folder_id
			WHERE c.depth < 64
		)
		SELECT * FROM chain ORDER BY depth`, folderID).Scan(&chain).Error
	return chain, err
}

func (r *courseFilesRepo) ContextUsage(ctx context.Context, contextType string, contextID uint) (int64, error) {
	var used int64
	err := r.db.WithContext(ctx).Model(&models.Attachment{}).
		Where("context_type = ? AND context_id = ? AND workflow_state <> ?", contextType, contextID, models.AttachmentDeleted).
		Select("COALESCE(SUM(size), 0)").Scan(&used).Error
	return used, err
}

func (r *courseFilesRepo) RefreshFolderPaths(ctx context.Context, folderID uint) error {
	return r.db.WithContext(ctx).Exec(`
		WITH RECURSIVE tree AS (
			SELECT id, full_name, 0 AS depth FROM folders WHERE id = ?
			UNION ALL
			SELECT f.id, tree.full_name || '/' || f.name, tree.depth + 1
			FROM folders f JOIN tree ON f.parent_folder_id = tree.id
			WHERE tree.depth < 64
		)
		UPDATE folders SET full_name = tree.full_name, updated_at = now()
		FROM tree WHERE folders.id = tree.id AND tree.depth > 0`, folderID).Error
}

func (r *courseFilesRepo) DeleteFolderTree(ctx context.Context, folderID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subtree := `WITH RECURSIVE tree AS (
			SELECT id FROM folders WHERE id = ?
			UNION
			SELECT f.id FROM folders f JOIN tree t ON f.parent_folder_id = t.id
		) SELECT id FROM tree`
		if err := tx.Exec(`UPDATE attachments SET workflow_state = ?, updated_at = now()
			WHERE folder_id IN (`+subtree+`) AND workflow_state <> ?`,
			models.AttachmentDeleted, folderID, models.AttachmentDeleted).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE folders SET workflow_state = 'deleted', updated_at = now() WHERE id IN (`+subtree+`)`, folderID).Error
	})
}

func (r *courseFilesRepo) NameTaken(ctx context.Context, contextType string, contextID uint, folderID *uint, displayName string, excludeID uint) (bool, error) {
	query := r.db.WithContext(ctx).Model(&models.Attachment{}).
		Where("context_type = ? AND context_id = ? AND workflow_state <> ? AND display_name = ? AND id <> ?",
			contextType, contextID, models.AttachmentDeleted, displayName, excludeID)
	if folderID == nil {
		query = query.Where("folder_id IS NULL")
	} else {
		query = query.Where("folder_id = ?", *folderID)
	}
	var n int64
	if err := query.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}
