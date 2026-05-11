package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type quizQuestionGroupRepo struct {
	db *gorm.DB
}

func NewQuizQuestionGroupRepository(db *gorm.DB) repository.QuizQuestionGroupRepository {
	return &quizQuestionGroupRepo{db: db}
}

func (r *quizQuestionGroupRepo) Create(ctx context.Context, group *models.QuizQuestionGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *quizQuestionGroupRepo) FindByID(ctx context.Context, id uint) (*models.QuizQuestionGroup, error) {
	var group models.QuizQuestionGroup
	if err := r.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *quizQuestionGroupRepo) Update(ctx context.Context, group *models.QuizQuestionGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *quizQuestionGroupRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.QuizQuestionGroup{}, id).Error
}

func (r *quizQuestionGroupRepo) ListByQuizID(ctx context.Context, quizID uint) ([]models.QuizQuestionGroup, error) {
	var groups []models.QuizQuestionGroup
	if err := r.db.WithContext(ctx).Where("quiz_id = ?", quizID).Order("position ASC, id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}
