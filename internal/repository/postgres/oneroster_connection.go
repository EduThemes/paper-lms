package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type oneRosterConnectionRepo struct {
	db *gorm.DB
}

func NewOneRosterConnectionRepository(db *gorm.DB) repository.OneRosterConnectionRepository {
	return &oneRosterConnectionRepo{db: db}
}

func (r *oneRosterConnectionRepo) Create(ctx context.Context, conn *models.OneRosterConnection) error {
	return r.db.WithContext(ctx).Create(conn).Error
}

func (r *oneRosterConnectionRepo) FindByID(ctx context.Context, id, accountID uint) (*models.OneRosterConnection, error) {
	var conn models.OneRosterConnection
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&conn, id).Error; err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *oneRosterConnectionRepo) Update(ctx context.Context, conn *models.OneRosterConnection) error {
	return r.db.WithContext(ctx).Save(conn).Error
}

func (r *oneRosterConnectionRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.OneRosterConnection{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *oneRosterConnectionRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.OneRosterConnection], error) {
	var connections []models.OneRosterConnection
	var count int64

	query := r.db.WithContext(ctx).Model(&models.OneRosterConnection{}).Where("account_id = ? AND workflow_state != ?", accountID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&connections).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.OneRosterConnection]{
		Items:      connections,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *oneRosterConnectionRepo) FindByAccountAndName(ctx context.Context, accountID uint, name string) (*models.OneRosterConnection, error) {
	var conn models.OneRosterConnection
	if err := r.db.WithContext(ctx).Where("account_id = ? AND name = ? AND workflow_state != ?", accountID, name, "deleted").First(&conn).Error; err != nil {
		return nil, err
	}
	return &conn, nil
}

func (r *oneRosterConnectionRepo) ListAutoSync(ctx context.Context) ([]models.OneRosterConnection, error) {
	var connections []models.OneRosterConnection
	if err := r.db.WithContext(ctx).Where("auto_sync = ? AND workflow_state = ?", true, "active").Find(&connections).Error; err != nil {
		return nil, err
	}
	return connections, nil
}
