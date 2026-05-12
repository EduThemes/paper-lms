package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GamificationFerpaFieldTagRepo persists the (object_type, field_path) →
// FERPA classification lookup that drives the data-access guard.
type GamificationFerpaFieldTagRepo struct {
	db *gorm.DB
}

func NewGamificationFerpaFieldTagRepository(db *gorm.DB) *GamificationFerpaFieldTagRepo {
	return &GamificationFerpaFieldTagRepo{db: db}
}

// Upsert writes or replaces the tag row. Identified by the composite PK.
func (r *GamificationFerpaFieldTagRepo) Upsert(ctx context.Context, tag *models.GamificationFerpaFieldTag) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "object_type"}, {Name: "field_path"}},
			DoUpdates: clause.AssignmentColumns([]string{"classification", "description", "updated_at"}),
		}).
		Create(tag).Error
}

func (r *GamificationFerpaFieldTagRepo) Find(ctx context.Context, objectType, fieldPath string) (*models.GamificationFerpaFieldTag, error) {
	var tag models.GamificationFerpaFieldTag
	err := r.db.WithContext(ctx).
		Where("object_type = ? AND field_path = ?", objectType, fieldPath).
		First(&tag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tag, nil
}

func (r *GamificationFerpaFieldTagRepo) ListByObjectType(ctx context.Context, objectType string) ([]models.GamificationFerpaFieldTag, error) {
	var tags []models.GamificationFerpaFieldTag
	err := r.db.WithContext(ctx).
		Where("object_type = ?", objectType).
		Order("field_path ASC").
		Find(&tags).Error
	return tags, err
}
