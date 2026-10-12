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

func (r *coursePaceModuleItemRepo) FindByID(ctx context.Context, id, accountID uint) (*models.CoursePaceModuleItem, error) {
	var item models.CoursePaceModuleItem
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Scope through pace→course→account_id.
		q = q.Where(`course_pace_id IN (
			SELECT id FROM course_paces WHERE course_id IN (
				SELECT id FROM courses WHERE account_id = ?
			)
		)`, accountID)
	}
	if err := q.First(&item, id).Error; err != nil {
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

// ListByPaceID returns the pace's items in COURSE order — module
// position, then item position within the module — which is the order
// the pacing schedule accumulates durations in (B143). The pre-fix
// ORDER BY module_item_id scheduled items by creation order, so an item
// added to Module 1 late (higher id) or a reordered item landed after
// everything in later modules. Items whose module item or module was
// deleted drop out of the schedule.
func (r *coursePaceModuleItemRepo) ListByPaceID(ctx context.Context, paceID uint) ([]models.CoursePaceModuleItem, error) {
	var items []models.CoursePaceModuleItem
	if err := r.db.WithContext(ctx).
		Select("course_pace_module_items.*").
		Joins("JOIN content_tags ON content_tags.id = course_pace_module_items.module_item_id").
		Joins("JOIN context_modules ON context_modules.id = content_tags.context_module_id").
		Where("course_pace_module_items.course_pace_id = ?", paceID).
		Where("content_tags.workflow_state <> ? AND context_modules.workflow_state <> ?", "deleted", "deleted").
		Order("context_modules.position ASC, context_modules.id ASC, content_tags.position ASC, content_tags.id ASC").
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
