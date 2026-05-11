package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// OutcomeProficiencyRepository persists and resolves proficiency scales.
type OutcomeProficiencyRepository struct {
	db *gorm.DB
}

func NewOutcomeProficiencyRepository(db *gorm.DB) *OutcomeProficiencyRepository {
	return &OutcomeProficiencyRepository{db: db}
}

// FindByContext returns the active proficiency scale for the given context (with
// ratings preloaded), or gorm.ErrRecordNotFound if none exists.
func (r *OutcomeProficiencyRepository) FindByContext(ctx context.Context, contextType string, contextID uint) (*models.OutcomeProficiency, error) {
	var p models.OutcomeProficiency
	err := r.db.WithContext(ctx).
		Preload("Ratings", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).
		Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted").
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *OutcomeProficiencyRepository) FindByID(ctx context.Context, id uint) (*models.OutcomeProficiency, error) {
	var p models.OutcomeProficiency
	err := r.db.WithContext(ctx).
		Preload("Ratings", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).
		First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Upsert creates or replaces a proficiency scale for the given context. Any prior
// scale is soft-deleted, ratings are wiped and re-inserted in one transaction.
func (r *OutcomeProficiencyRepository) Upsert(ctx context.Context, contextType string, contextID uint, ratings []models.OutcomeProficiencyRating) (*models.OutcomeProficiency, error) {
	var result *models.OutcomeProficiency
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.OutcomeProficiency
		err := tx.Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted").First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			existing = models.OutcomeProficiency{ContextType: contextType, ContextID: contextID, WorkflowState: "active"}
			if err := tx.Create(&existing).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Where("outcome_proficiency_id = ?", existing.ID).Delete(&models.OutcomeProficiencyRating{}).Error; err != nil {
				return err
			}
		}
		for i := range ratings {
			ratings[i].ID = 0
			ratings[i].OutcomeProficiencyID = existing.ID
			if ratings[i].Position == 0 {
				ratings[i].Position = i + 1
			}
		}
		if len(ratings) > 0 {
			if err := tx.Create(&ratings).Error; err != nil {
				return err
			}
		}
		existing.Ratings = ratings
		result = &existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Delete soft-deletes the proficiency scale for the given context.
func (r *OutcomeProficiencyRepository) Delete(ctx context.Context, contextType string, contextID uint) error {
	return r.db.WithContext(ctx).Model(&models.OutcomeProficiency{}).
		Where("context_type = ? AND context_id = ?", contextType, contextID).
		Update("workflow_state", "deleted").Error
}

// ResolveForCourse returns the proficiency scale to apply to a course. Resolution
// order: course-level -> account-level (via course.account_id) -> system default.
// The returned proficiency may have ID == 0 if it is the system-default fallback.
func (r *OutcomeProficiencyRepository) ResolveForCourse(ctx context.Context, courseID uint) (*models.OutcomeProficiency, error) {
	if p, err := r.FindByContext(ctx, "Course", courseID); err == nil {
		return p, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// fall back to course's account
	var course models.Course
	if err := r.db.WithContext(ctx).Select("id", "account_id").First(&course, courseID).Error; err == nil && course.AccountID != 0 {
		if p, err := r.FindByContext(ctx, "Account", course.AccountID); err == nil {
			return p, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	// system default
	return &models.OutcomeProficiency{
		ContextType:   "System",
		WorkflowState: "active",
		Ratings:       models.DefaultProficiencyRatings(),
	}, nil
}
