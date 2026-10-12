package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/security/htmlsanitize"
	"gorm.io/gorm"
)

// quizItemBankItemRepo serves item-bank items from question_bank_entries
// (the single question-bank store, G060). Items of deleted banks are not
// found.
type quizItemBankItemRepo struct {
	db *gorm.DB
}

func NewQuizItemBankItemRepository(db *gorm.DB) *quizItemBankItemRepo {
	return &quizItemBankItemRepo{db: db}
}

func itemFromEntry(e *models.QuestionBankEntry) *models.QuizItemBankItem {
	pts := e.PointsPossible
	return &models.QuizItemBankItem{
		ID:                e.ID,
		BankID:            e.QuestionBankID,
		Position:          e.Position,
		QuestionType:      e.QuestionType,
		QuestionText:      e.QuestionText,
		PointsPossible:    &pts,
		Answers:           e.Answers,
		CorrectComments:   e.CorrectComments,
		IncorrectComments: e.IncorrectComments,
		NeutralComments:   e.NeutralComments,
		CreatedAt:         e.CreatedAt,
		UpdatedAt:         e.UpdatedAt,
	}
}

func entryFromItem(item *models.QuizItemBankItem) *models.QuestionBankEntry {
	pts := 1.0
	if item.PointsPossible != nil {
		pts = *item.PointsPossible
	}
	answers := item.Answers
	if answers == "" {
		answers = "[]"
	}
	return &models.QuestionBankEntry{
		ID:                item.ID,
		QuestionBankID:    item.BankID,
		QuestionType:      item.QuestionType,
		QuestionText:      item.QuestionText,
		PointsPossible:    pts,
		Answers:           answers,
		Position:          item.Position,
		CorrectComments:   item.CorrectComments,
		IncorrectComments: item.IncorrectComments,
		NeutralComments:   item.NeutralComments,
	}
}

// liveEntries restricts to entries whose bank is not deleted.
func (r *quizItemBankItemRepo) liveEntries(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Model(&models.QuestionBankEntry{}).
		Where("question_bank_id IN (SELECT id FROM question_banks WHERE workflow_state <> 'deleted')")
}

func (r *quizItemBankItemRepo) Create(ctx context.Context, item *models.QuizItemBankItem) error {
	e := entryFromItem(item)
	e.ID = 0
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return err
	}
	item.ID, item.CreatedAt, item.UpdatedAt = e.ID, e.CreatedAt, e.UpdatedAt
	return nil
}

func (r *quizItemBankItemRepo) FindByID(ctx context.Context, id uint) (*models.QuizItemBankItem, error) {
	var e models.QuestionBankEntry
	if err := r.liveEntries(ctx).First(&e, id).Error; err != nil {
		return nil, err
	}
	return itemFromEntry(&e), nil
}

func (r *quizItemBankItemRepo) Update(ctx context.Context, item *models.QuizItemBankItem) error {
	e := entryFromItem(item)
	// Updates(map) skips the model's BeforeSave hook, so the rich-content
	// columns are sanitized here.
	res := r.db.WithContext(ctx).Model(&models.QuestionBankEntry{}).Where("id = ?", item.ID).
		Updates(map[string]interface{}{
			"question_bank_id":   e.QuestionBankID,
			"question_type":      e.QuestionType,
			"question_text":      htmlsanitize.Sanitize(e.QuestionText),
			"points_possible":    e.PointsPossible,
			"answers":            e.Answers,
			"position":           e.Position,
			"correct_comments":   htmlsanitize.Sanitize(e.CorrectComments),
			"incorrect_comments": htmlsanitize.Sanitize(e.IncorrectComments),
			"neutral_comments":   htmlsanitize.Sanitize(e.NeutralComments),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *quizItemBankItemRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.QuestionBankEntry{}, id).Error
}

func (r *quizItemBankItemRepo) ListByBankID(ctx context.Context, bankID uint) ([]models.QuizItemBankItem, error) {
	var entries []models.QuestionBankEntry
	if err := r.liveEntries(ctx).
		Where("question_bank_id = ?", bankID).
		Order("position ASC, id ASC").
		Find(&entries).Error; err != nil {
		return nil, err
	}
	out := make([]models.QuizItemBankItem, len(entries))
	for i := range entries {
		out[i] = *itemFromEntry(&entries[i])
	}
	return out, nil
}
