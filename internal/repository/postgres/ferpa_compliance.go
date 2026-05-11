package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// DataRetentionPolicyRepository defines the data access methods for data retention policies.
type DataRetentionPolicyRepository interface {
	Create(ctx context.Context, policy *models.DataRetentionPolicy) error
	FindByID(ctx context.Context, id uint) (*models.DataRetentionPolicy, error)
	Update(ctx context.Context, policy *models.DataRetentionPolicy) error
	Delete(ctx context.Context, id uint) error
	ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DataRetentionPolicy], error)
}

type dataRetentionPolicyRepo struct {
	db *gorm.DB
}

// NewDataRetentionPolicyRepository creates a new DataRetentionPolicy repository backed by PostgreSQL.
func NewDataRetentionPolicyRepository(db *gorm.DB) DataRetentionPolicyRepository {
	return &dataRetentionPolicyRepo{db: db}
}

func (r *dataRetentionPolicyRepo) Create(ctx context.Context, policy *models.DataRetentionPolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

func (r *dataRetentionPolicyRepo) FindByID(ctx context.Context, id uint) (*models.DataRetentionPolicy, error) {
	var policy models.DataRetentionPolicy
	if err := r.db.WithContext(ctx).First(&policy, id).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *dataRetentionPolicyRepo) Update(ctx context.Context, policy *models.DataRetentionPolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *dataRetentionPolicyRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.DataRetentionPolicy{}, id).Error
}

func (r *dataRetentionPolicyRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DataRetentionPolicy], error) {
	var policies []models.DataRetentionPolicy
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DataRetentionPolicy{}).Where("account_id = ?", accountID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&policies).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DataRetentionPolicy]{
		Items:      policies,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// DataDeletionRequestRepository defines the data access methods for data deletion requests.
type DataDeletionRequestRepository interface {
	Create(ctx context.Context, request *models.DataDeletionRequest) error
	FindByID(ctx context.Context, id uint) (*models.DataDeletionRequest, error)
	Update(ctx context.Context, request *models.DataDeletionRequest) error
	ListPending(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.DataDeletionRequest], error)
	ListByUserID(ctx context.Context, userID uint) ([]models.DataDeletionRequest, error)
}

type dataDeletionRequestRepo struct {
	db *gorm.DB
}

// NewDataDeletionRequestRepository creates a new DataDeletionRequest repository backed by PostgreSQL.
func NewDataDeletionRequestRepository(db *gorm.DB) DataDeletionRequestRepository {
	return &dataDeletionRequestRepo{db: db}
}

func (r *dataDeletionRequestRepo) Create(ctx context.Context, request *models.DataDeletionRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}

func (r *dataDeletionRequestRepo) FindByID(ctx context.Context, id uint) (*models.DataDeletionRequest, error) {
	var request models.DataDeletionRequest
	if err := r.db.WithContext(ctx).First(&request, id).Error; err != nil {
		return nil, err
	}
	return &request, nil
}

func (r *dataDeletionRequestRepo) Update(ctx context.Context, request *models.DataDeletionRequest) error {
	return r.db.WithContext(ctx).Save(request).Error
}

func (r *dataDeletionRequestRepo) ListPending(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.DataDeletionRequest], error) {
	var requests []models.DataDeletionRequest
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DataDeletionRequest{}).Where("status = ?", "pending")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&requests).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DataDeletionRequest]{
		Items:      requests,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *dataDeletionRequestRepo) ListByUserID(ctx context.Context, userID uint) ([]models.DataDeletionRequest, error) {
	var requests []models.DataDeletionRequest
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&requests).Error; err != nil {
		return nil, err
	}
	return requests, nil
}

// DataExportRequestRepository defines the data access methods for data export requests.
type DataExportRequestRepository interface {
	Create(ctx context.Context, request *models.DataExportRequest) error
	FindByID(ctx context.Context, id uint) (*models.DataExportRequest, error)
	Update(ctx context.Context, request *models.DataExportRequest) error
	ListByUserID(ctx context.Context, userID uint) ([]models.DataExportRequest, error)
}

type dataExportRequestRepo struct {
	db *gorm.DB
}

// NewDataExportRequestRepository creates a new DataExportRequest repository backed by PostgreSQL.
func NewDataExportRequestRepository(db *gorm.DB) DataExportRequestRepository {
	return &dataExportRequestRepo{db: db}
}

func (r *dataExportRequestRepo) Create(ctx context.Context, request *models.DataExportRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}

func (r *dataExportRequestRepo) FindByID(ctx context.Context, id uint) (*models.DataExportRequest, error) {
	var request models.DataExportRequest
	if err := r.db.WithContext(ctx).First(&request, id).Error; err != nil {
		return nil, err
	}
	return &request, nil
}

func (r *dataExportRequestRepo) Update(ctx context.Context, request *models.DataExportRequest) error {
	return r.db.WithContext(ctx).Save(request).Error
}

func (r *dataExportRequestRepo) ListByUserID(ctx context.Context, userID uint) ([]models.DataExportRequest, error) {
	var requests []models.DataExportRequest
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&requests).Error; err != nil {
		return nil, err
	}
	return requests, nil
}

// PIIAccessLogRepository defines the data access methods for PII access audit logs.
type PIIAccessLogRepository interface {
	Create(ctx context.Context, log *models.PIIAccessLog) error
	ListByStudentID(ctx context.Context, studentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error)
	ListByAccessorID(ctx context.Context, accessorID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error)
}

type piiAccessLogRepo struct {
	db *gorm.DB
}

// NewPIIAccessLogRepository creates a new PIIAccessLog repository backed by PostgreSQL.
func NewPIIAccessLogRepository(db *gorm.DB) PIIAccessLogRepository {
	return &piiAccessLogRepo{db: db}
}

func (r *piiAccessLogRepo) Create(ctx context.Context, log *models.PIIAccessLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *piiAccessLogRepo) ListByStudentID(ctx context.Context, studentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	var logs []models.PIIAccessLog
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PIIAccessLog{}).Where("student_id = ?", studentID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PIIAccessLog]{
		Items:      logs,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *piiAccessLogRepo) ListByAccessorID(ctx context.Context, accessorID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PIIAccessLog], error) {
	var logs []models.PIIAccessLog
	var count int64

	query := r.db.WithContext(ctx).Model(&models.PIIAccessLog{}).Where("accessor_id = ?", accessorID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.PIIAccessLog]{
		Items:      logs,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
