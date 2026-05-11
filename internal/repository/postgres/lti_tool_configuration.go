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

func (r *ltiToolConfigurationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.LTIToolConfiguration{}, id).Error
}
