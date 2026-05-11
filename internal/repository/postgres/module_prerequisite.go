package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type modulePrerequisiteRepo struct {
	db *gorm.DB
}

func NewModulePrerequisiteRepository(db *gorm.DB) repository.ModulePrerequisiteRepository {
	return &modulePrerequisiteRepo{db: db}
}

func (r *modulePrerequisiteRepo) SetPrerequisites(ctx context.Context, moduleID uint, prerequisiteModuleIDs []uint) error {
	// Delete existing prerequisites
	if err := r.db.WithContext(ctx).Where("module_id = ?", moduleID).Delete(&models.ModulePrerequisite{}).Error; err != nil {
		return err
	}

	// Create new prerequisites
	for _, prereqID := range prerequisiteModuleIDs {
		mp := models.ModulePrerequisite{
			ModuleID:             moduleID,
			PrerequisiteModuleID: prereqID,
		}
		if err := r.db.WithContext(ctx).Create(&mp).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *modulePrerequisiteRepo) GetPrerequisites(ctx context.Context, moduleID uint) ([]uint, error) {
	var prereqs []models.ModulePrerequisite
	if err := r.db.WithContext(ctx).Where("module_id = ?", moduleID).Find(&prereqs).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, len(prereqs))
	for i, p := range prereqs {
		ids[i] = p.PrerequisiteModuleID
	}
	return ids, nil
}

func (r *modulePrerequisiteRepo) GetModulesWithPrerequisite(ctx context.Context, prerequisiteModuleID uint) ([]uint, error) {
	var prereqs []models.ModulePrerequisite
	if err := r.db.WithContext(ctx).Where("prerequisite_module_id = ?", prerequisiteModuleID).Find(&prereqs).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, len(prereqs))
	for i, p := range prereqs {
		ids[i] = p.ModuleID
	}
	return ids, nil
}
