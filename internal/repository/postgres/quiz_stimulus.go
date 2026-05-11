package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type quizStimulusRepo struct {
	db *gorm.DB
}

func NewQuizStimulusRepository(db *gorm.DB) *quizStimulusRepo {
	return &quizStimulusRepo{db: db}
}

func (r *quizStimulusRepo) Create(ctx context.Context, s *models.QuizStimulus) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *quizStimulusRepo) FindByID(ctx context.Context, id uint) (*models.QuizStimulus, error) {
	var s models.QuizStimulus
	if err := r.db.WithContext(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *quizStimulusRepo) Update(ctx context.Context, s *models.QuizStimulus) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *quizStimulusRepo) Delete(ctx context.Context, id uint) error {
	// Null out the FK on any quiz_questions pointing at this stimulus first.
	if err := r.db.WithContext(ctx).
		Model(&models.QuizQuestion{}).
		Where("stimulus_id = ?", id).
		Update("stimulus_id", nil).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&models.QuizStimulus{}, id).Error
}

func (r *quizStimulusRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizStimulus], error) {
	var items []models.QuizStimulus
	var total int64

	query := r.db.WithContext(ctx).Model(&models.QuizStimulus{}).Where("course_id = ?", courseID)
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.QuizStimulus]{
		Items:      items,
		TotalCount: total,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *quizStimulusRepo) ListQuestionsForStimulus(ctx context.Context, stimulusID uint) ([]models.QuizQuestion, error) {
	var questions []models.QuizQuestion
	if err := r.db.WithContext(ctx).
		Where("stimulus_id = ? AND workflow_state != ?", stimulusID, "deleted").
		Order("position ASC, id ASC").
		Find(&questions).Error; err != nil {
		return nil, err
	}
	return questions, nil
}

func (r *quizStimulusRepo) SetQuestionStimulus(ctx context.Context, questionID uint, stimulusID *uint) error {
	return r.db.WithContext(ctx).
		Model(&models.QuizQuestion{}).
		Where("id = ?", questionID).
		Update("stimulus_id", stimulusID).Error
}
