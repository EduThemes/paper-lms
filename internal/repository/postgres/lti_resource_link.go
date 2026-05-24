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

// Delete — F-012 widening: tenant-scoped via the context_external_tool →
// developer_keys.account_id chain. Mirrors the polymorphic context_type
// guard used in ContextExternalToolRepository.FindByID — accepted scope
// is "the caller's tenant owns the parent ContextExternalTool, which
// itself is scoped through Course→account_id or Account→account_id."
// accountID==0 skips the scope filter (auth-internal callers only);
// handler callers MUST pass callerAccountID(c).
func (r *ltiResourceLinkRepo) Delete(ctx context.Context, id, accountID uint) error {
	q := r.db.WithContext(ctx).Model(&models.LTIResourceLink{}).Where("id = ?", id)
	if accountID != 0 {
		q = q.Where(`context_external_tool_id IN (
			SELECT cet.id FROM context_external_tools cet
			WHERE (cet.context_type = 'Course'
			       AND cet.context_id IN (SELECT id FROM courses WHERE account_id = ?))
			   OR (cet.context_type = 'Account' AND cet.context_id = ?)
		)`, accountID, accountID)
	}
	return q.Delete(&models.LTIResourceLink{}).Error
}
