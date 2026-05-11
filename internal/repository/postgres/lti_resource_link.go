package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type ltiResourceLinkRepo struct {
	db *gorm.DB
}

func NewLTIResourceLinkRepository(db *gorm.DB) repository.LTIResourceLinkRepository {
	return &ltiResourceLinkRepo{db: db}
}

func (r *ltiResourceLinkRepo) Create(ctx context.Context, link *models.LTIResourceLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *ltiResourceLinkRepo) FindByID(ctx context.Context, id uint) (*models.LTIResourceLink, error) {
	var link models.LTIResourceLink
	if err := r.db.WithContext(ctx).First(&link, id).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *ltiResourceLinkRepo) FindByResourceLinkID(ctx context.Context, resourceLinkID string) (*models.LTIResourceLink, error) {
	var link models.LTIResourceLink
	if err := r.db.WithContext(ctx).Where("resource_link_id = ?", resourceLinkID).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *ltiResourceLinkRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LTIResourceLink{}, id).Error
}
