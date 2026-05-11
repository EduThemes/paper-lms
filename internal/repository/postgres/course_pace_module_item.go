package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type coursePaceModuleItemRepo struct {
	db *gorm.DB
}

func NewCoursePaceModuleItemRepository(db *gorm.DB) repository.CoursePaceModuleItemRepository {
	return &coursePaceModuleItemRepo{db: db}
}

func (r *coursePaceModuleItemRepo) Create(ctx context.Context, item *models.CoursePaceModuleItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *coursePaceModuleItemRepo) FindByID(ctx context.Context, id uint) (*models.CoursePaceModuleItem, error) {
	var item models.CoursePaceModuleItem
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *coursePaceModuleItemRepo) Update(ctx context.Context, item *models.CoursePaceModuleItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *coursePaceModuleItemRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.CoursePaceModuleItem{}, id).Error
}

func (r *coursePaceModuleItemRepo) ListByPaceID(ctx context.Context, paceID uint) ([]models.CoursePaceModuleItem, error) {
	var items []models.CoursePaceModuleItem
	if err := r.db.WithContext(ctx).
		Where("course_pace_id = ?", paceID).
		Order("module_item_id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *coursePaceModuleItemRepo) BulkUpsert(ctx context.Context, items []models.CoursePaceModuleItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "course_pace_id"}, {Name: "module_item_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"duration", "updated_at"}),
		}).
		Create(&items).Error
}
