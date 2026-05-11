package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type rubricAssociationRepo struct {
	db *gorm.DB
}

func NewRubricAssociationRepository(db *gorm.DB) repository.RubricAssociationRepository {
	return &rubricAssociationRepo{db: db}
}

func (r *rubricAssociationRepo) Create(ctx context.Context, assoc *models.RubricAssociation) error {
	return r.db.WithContext(ctx).Create(assoc).Error
}

func (r *rubricAssociationRepo) FindByID(ctx context.Context, id uint) (*models.RubricAssociation, error) {
	var assoc models.RubricAssociation
	if err := r.db.WithContext(ctx).First(&assoc, id).Error; err != nil {
		return nil, err
	}
	return &assoc, nil
}

func (r *rubricAssociationRepo) Update(ctx context.Context, assoc *models.RubricAssociation) error {
	return r.db.WithContext(ctx).Save(assoc).Error
}

func (r *rubricAssociationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.RubricAssociation{}, id).Error
}

func (r *rubricAssociationRepo) FindByAssociation(ctx context.Context, associationID uint, associationType string) (*models.RubricAssociation, error) {
	var assoc models.RubricAssociation
	if err := r.db.WithContext(ctx).Where("association_id = ? AND association_type = ?", associationID, associationType).First(&assoc).Error; err != nil {
		return nil, err
	}
	return &assoc, nil
}
