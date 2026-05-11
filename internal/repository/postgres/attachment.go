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

func (r *attachmentRepo) FindByID(ctx context.Context, id uint) (*models.Attachment, error) {
	var attachment models.Attachment
	if err := r.db.WithContext(ctx).First(&attachment, id).Error; err != nil {
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
