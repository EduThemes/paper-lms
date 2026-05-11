package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type latePolicyRepo struct {
	db *gorm.DB
}

func NewLatePolicyRepository(db *gorm.DB) repository.LatePolicyRepository {
	return &latePolicyRepo{db: db}
}

func (r *latePolicyRepo) Create(ctx context.Context, policy *models.LatePolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

func (r *latePolicyRepo) FindByCourseID(ctx context.Context, courseID uint) (*models.LatePolicy, error) {
	var policy models.LatePolicy
	if err := r.db.WithContext(ctx).Where("course_id = ?", courseID).First(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *latePolicyRepo) Update(ctx context.Context, policy *models.LatePolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *latePolicyRepo) Delete(ctx context.Context, courseID uint) error {
	return r.db.WithContext(ctx).Where("course_id = ?", courseID).Delete(&models.LatePolicy{}).Error
}
