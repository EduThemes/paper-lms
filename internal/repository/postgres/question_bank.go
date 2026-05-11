package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type questionBankRepo struct {
	db *gorm.DB
}

func NewQuestionBankRepository(db *gorm.DB) repository.QuestionBankRepository {
	return &questionBankRepo{db: db}
}

func (r *questionBankRepo) Create(ctx context.Context, qb *models.QuestionBank) error {
	return r.db.WithContext(ctx).Create(qb).Error
}

func (r *questionBankRepo) FindByID(ctx context.Context, id uint) (*models.QuestionBank, error) {
	var qb models.QuestionBank
	if err := r.db.WithContext(ctx).First(&qb, id).Error; err != nil {
		return nil, err
	}
	return &qb, nil
}

func (r *questionBankRepo) Update(ctx context.Context, qb *models.QuestionBank) error {
	return r.db.WithContext(ctx).Save(qb).Error
}

func (r *questionBankRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.QuestionBank{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *questionBankRepo) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuestionBank], error) {
	var banks []models.QuestionBank
	var count int64

	query := r.db.WithContext(ctx).Model(&models.QuestionBank{}).Where("course_id = ? AND workflow_state = ?", courseID, "active")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&banks).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.QuestionBank]{
		Items:      banks,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// Question Bank Entries

type questionBankEntryRepo struct {
	db *gorm.DB
}

func NewQuestionBankEntryRepository(db *gorm.DB) repository.QuestionBankEntryRepository {
	return &questionBankEntryRepo{db: db}
}

func (r *questionBankEntryRepo) Create(ctx context.Context, entry *models.QuestionBankEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *questionBankEntryRepo) FindByID(ctx context.Context, id uint) (*models.QuestionBankEntry, error) {
	var entry models.QuestionBankEntry
	if err := r.db.WithContext(ctx).First(&entry, id).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *questionBankEntryRepo) Update(ctx context.Context, entry *models.QuestionBankEntry) error {
	return r.db.WithContext(ctx).Save(entry).Error
}

func (r *questionBankEntryRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.QuestionBankEntry{}, id).Error
}

func (r *questionBankEntryRepo) ListByBankID(ctx context.Context, bankID uint) ([]models.QuestionBankEntry, error) {
	var entries []models.QuestionBankEntry
	if err := r.db.WithContext(ctx).Where("question_bank_id = ?", bankID).Order("position ASC, id ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
