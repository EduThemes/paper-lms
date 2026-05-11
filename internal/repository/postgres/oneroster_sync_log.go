package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type oneRosterSyncLogRepo struct {
	db *gorm.DB
}

func NewOneRosterSyncLogRepository(db *gorm.DB) repository.OneRosterSyncLogRepository {
	return &oneRosterSyncLogRepo{db: db}
}

func (r *oneRosterSyncLogRepo) Create(ctx context.Context, log *models.OneRosterSyncLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *oneRosterSyncLogRepo) Update(ctx context.Context, log *models.OneRosterSyncLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

func (r *oneRosterSyncLogRepo) ListByConnectionID(ctx context.Context, connectionID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.OneRosterSyncLog], error) {
	var logs []models.OneRosterSyncLog
	var count int64

	query := r.db.WithContext(ctx).Model(&models.OneRosterSyncLog{}).Where("connection_id = ?", connectionID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.OneRosterSyncLog]{
		Items:      logs,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *oneRosterSyncLogRepo) GetLatestByConnectionID(ctx context.Context, connectionID uint) (*models.OneRosterSyncLog, error) {
	var log models.OneRosterSyncLog
	if err := r.db.WithContext(ctx).Where("connection_id = ?", connectionID).Order("id DESC").First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}
