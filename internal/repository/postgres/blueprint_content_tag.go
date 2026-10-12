package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type blueprintContentTagRepo struct {
	db *gorm.DB
}

func NewBlueprintContentTagRepository(db *gorm.DB) repository.BlueprintContentTagRepository {
	return &blueprintContentTagRepo{db: db}
}

func (r *blueprintContentTagRepo) Find(ctx context.Context, templateID, childCourseID uint, contentType string, sourceID uint) (*models.BlueprintContentTag, error) {
	var tag models.BlueprintContentTag
	err := r.db.WithContext(ctx).
		Where("blueprint_template_id = ? AND child_course_id = ? AND content_type = ? AND source_id = ?",
			templateID, childCourseID, contentType, sourceID).
		First(&tag).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *blueprintContentTagRepo) Upsert(ctx context.Context, tag *models.BlueprintContentTag) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "blueprint_template_id"}, {Name: "child_course_id"},
				{Name: "content_type"}, {Name: "source_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"child_id", "synced_at", "updated_at"}),
		}).
		Create(tag).Error
}

func (r *blueprintContentTagRepo) ListByChildCourse(ctx context.Context, templateID, childCourseID uint) ([]models.BlueprintContentTag, error) {
	var tags []models.BlueprintContentTag
	err := r.db.WithContext(ctx).
		Where("blueprint_template_id = ? AND child_course_id = ?", templateID, childCourseID).
		Order("id").
		Find(&tags).Error
	return tags, err
}

func (r *blueprintContentTagRepo) FindByChildItem(ctx context.Context, childCourseID uint, contentType string, childID uint) (*models.BlueprintContentTag, error) {
	var tag models.BlueprintContentTag
	err := r.db.WithContext(ctx).
		Where("child_course_id = ? AND content_type = ? AND child_id = ?", childCourseID, contentType, childID).
		Order("id DESC").
		First(&tag).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *blueprintContentTagRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.BlueprintContentTag{}, id).Error
}
