package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type attachmentRepo struct {
	db *gorm.DB
}

func NewAttachmentRepository(db *gorm.DB) repository.AttachmentRepository {
	return &attachmentRepo{db: db}
}

func (r *attachmentRepo) Create(ctx context.Context, attachment *models.Attachment) error {
	return r.db.WithContext(ctx).Create(attachment).Error
}

func (r *attachmentRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Attachment, error) {
	var attachment models.Attachment
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Attachments are polymorphic on (context_type, context_id) AND
		// optionally chained to a folder. We tenant-filter by the same
		// polymorphic branches we use on folders.
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
	if err := q.First(&attachment, id).Error; err != nil {
		return nil, err
	}
	return &attachment, nil
}

func (r *attachmentRepo) Update(ctx context.Context, attachment *models.Attachment) error {
	return r.db.WithContext(ctx).Save(attachment).Error
}

func (r *attachmentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Attachment{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *attachmentRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	var attachments []models.Attachment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Attachment{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("display_name ASC").Find(&attachments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Attachment]{
		Items:      attachments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *attachmentRepo) ListByFolderID(ctx context.Context, folderID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	var attachments []models.Attachment
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Attachment{}).Where("folder_id = ? AND workflow_state != ?", folderID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("display_name ASC").Find(&attachments).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Attachment]{
		Items:      attachments,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
