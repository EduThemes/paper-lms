package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type accountRepo struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) repository.AccountRepository {
	return &accountRepo{db: db}
}

func (r *accountRepo) Create(ctx context.Context, account *models.Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *accountRepo) FindByID(ctx context.Context, id uint) (*models.Account, error) {
	var account models.Account
	if err := r.db.WithContext(ctx).First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *accountRepo) Update(ctx context.Context, account *models.Account) error {
	return r.db.WithContext(ctx).Save(account).Error
}

func (r *accountRepo) List(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Account], error) {
	var accounts []models.Account
	var count int64

	r.db.WithContext(ctx).Model(&models.Account{}).Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := r.db.WithContext(ctx).Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&accounts).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Account]{
		Items:      accounts,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
