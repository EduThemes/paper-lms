package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type ltiToolConfigurationRepo struct {
	db *gorm.DB
}

func NewLTIToolConfigurationRepository(db *gorm.DB) repository.LTIToolConfigurationRepository {
	return &ltiToolConfigurationRepo{db: db}
}

func (r *ltiToolConfigurationRepo) Create(ctx context.Context, config *models.LTIToolConfiguration) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *ltiToolConfigurationRepo) FindByID(ctx context.Context, id uint) (*models.LTIToolConfiguration, error) {
	var config models.LTIToolConfiguration
	if err := r.db.WithContext(ctx).First(&config, id).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ltiToolConfigurationRepo) FindByDeveloperKeyID(ctx context.Context, devKeyID uint) (*models.LTIToolConfiguration, error) {
	var config models.LTIToolConfiguration
	if err := r.db.WithContext(ctx).Where("developer_key_id = ?", devKeyID).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ltiToolConfigurationRepo) Update(ctx context.Context, config *models.LTIToolConfiguration) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// Delete — F-012 widening: tenant-scoped via developer_keys.account_id.
// LTIToolConfiguration has a 1:1 developer_key_id FK; the developer key
// carries account_id. accountID==0 skips the scope filter (auth-internal
// callers only); handler callers MUST pass callerAccountID(c).
func (r *ltiToolConfigurationRepo) Delete(ctx context.Context, id, accountID uint) error {
	q := r.db.WithContext(ctx).Model(&models.LTIToolConfiguration{}).Where("id = ?", id)
	if accountID != 0 {
		q = q.Where(`developer_key_id IN (
			SELECT id FROM developer_keys WHERE account_id = ?
		)`, accountID)
	}
	return q.Delete(&models.LTIToolConfiguration{}).Error
}
