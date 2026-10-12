package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// quizItemBankRepo serves the /quiz_item_banks API from the single
// question-bank store (question_banks), so the item-bank page, the QTI
// importer, question groups and the course import all see the same banks
// (G060, migration 000091). The QuizItemBank model is only the API shape.
type quizItemBankRepo struct {
	db *gorm.DB
}

func NewQuizItemBankRepository(db *gorm.DB) *quizItemBankRepo {
	return &quizItemBankRepo{db: db}
}

func itemBankFromQuestionBank(b *models.QuestionBank) *models.QuizItemBank {
	out := &models.QuizItemBank{
		ID:          b.ID,
		CourseID:    b.CourseID,
		Title:       b.Title,
		Description: b.Description,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
	if b.CreatedByUserID != nil {
		out.CreatedByUserID = *b.CreatedByUserID
	}
	return out
}

func (r *quizItemBankRepo) Create(ctx context.Context, bank *models.QuizItemBank) error {
	creator := bank.CreatedByUserID
	qb := &models.QuestionBank{
		CourseID:        bank.CourseID,
		Title:           bank.Title,
		Description:     bank.Description,
		CreatedByUserID: &creator,
		WorkflowState:   "active",
	}
	if err := NewQuestionBankRepository(r.db).Create(ctx, qb); err != nil {
		return err
	}
	bank.ID, bank.CreatedAt, bank.UpdatedAt = qb.ID, qb.CreatedAt, qb.UpdatedAt
	return nil
}

// FindByID returns an active bank; deleted banks are not found.
func (r *quizItemBankRepo) FindByID(ctx context.Context, id uint) (*models.QuizItemBank, error) {
	var qb models.QuestionBank
	if err := r.db.WithContext(ctx).Where("workflow_state <> ?", "deleted").First(&qb, id).Error; err != nil {
		return nil, err
	}
	return itemBankFromQuestionBank(&qb), nil
}

func (r *quizItemBankRepo) Update(ctx context.Context, bank *models.QuizItemBank) error {
	res := r.db.WithContext(ctx).Model(&models.QuestionBank{}).
		Where("id = ? AND workflow_state <> ?", bank.ID, "deleted").
		Updates(map[string]interface{}{"title": bank.Title, "description": bank.Description})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete soft-deletes the bank (question groups linked to it stop drawing).
func (r *quizItemBankRepo) Delete(ctx context.Context, id uint) error {
	return NewQuestionBankRepository(r.db).Delete(ctx, id)
}

func (r *quizItemBankRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizItemBank], error) {
	var banks []models.QuestionBank
	var total int64
	query := r.db.WithContext(ctx).Model(&models.QuestionBank{}).
		Where("course_id = ? AND workflow_state = ?", courseID, "active")
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(params.PerPage).Find(&banks).Error; err != nil {
		return nil, err
	}
	items := make([]models.QuizItemBank, len(banks))
	for i := range banks {
		items[i] = *itemBankFromQuestionBank(&banks[i])
	}
	return &repository.PaginatedResult[models.QuizItemBank]{
		Items:      items,
		TotalCount: total,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
