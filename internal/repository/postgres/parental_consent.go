package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// ParentalConsentRepository defines the data access methods for parental consent records.
type ParentalConsentRepository interface {
	Create(ctx context.Context, consent *models.ParentalConsent) error
	FindByID(ctx context.Context, id uint) (*models.ParentalConsent, error)
	FindByStudentID(ctx context.Context, studentID uint) ([]models.ParentalConsent, error)
	FindByToken(ctx context.Context, token string) (*models.ParentalConsent, error)
	Update(ctx context.Context, consent *models.ParentalConsent) error
	ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ParentalConsent], error)
}

type parentalConsentRepo struct {
	db *gorm.DB
}

// NewParentalConsentRepository creates a new ParentalConsent repository backed by PostgreSQL.
func NewParentalConsentRepository(db *gorm.DB) ParentalConsentRepository {
	return &parentalConsentRepo{db: db}
}

func (r *parentalConsentRepo) Create(ctx context.Context, consent *models.ParentalConsent) error {
	return r.db.WithContext(ctx).Create(consent).Error
}

func (r *parentalConsentRepo) FindByID(ctx context.Context, id uint) (*models.ParentalConsent, error) {
	var consent models.ParentalConsent
	if err := r.db.WithContext(ctx).First(&consent, id).Error; err != nil {
		return nil, err
	}
	return &consent, nil
}

func (r *parentalConsentRepo) FindByStudentID(ctx context.Context, studentID uint) ([]models.ParentalConsent, error) {
	var consents []models.ParentalConsent
	if err := r.db.WithContext(ctx).Where("student_id = ?", studentID).Order("created_at DESC").Find(&consents).Error; err != nil {
		return nil, err
	}
	return consents, nil
}

func (r *parentalConsentRepo) FindByToken(ctx context.Context, token string) (*models.ParentalConsent, error) {
	var consent models.ParentalConsent
	if err := r.db.WithContext(ctx).Where("verification_token = ?", token).First(&consent).Error; err != nil {
		return nil, err
	}
	return &consent, nil
}

func (r *parentalConsentRepo) Update(ctx context.Context, consent *models.ParentalConsent) error {
	return r.db.WithContext(ctx).Save(consent).Error
}

func (r *parentalConsentRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ParentalConsent], error) {
	var consents []models.ParentalConsent
	var count int64

	// Join through the student's account to filter by account_id
	query := r.db.WithContext(ctx).Model(&models.ParentalConsent{}).
		Joins("JOIN users ON users.id = parental_consents.student_id").
		Where("users.account_id = ?", accountID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("parental_consents.created_at DESC").Find(&consents).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.ParentalConsent]{
		Items:      consents,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// DataProcessingAgreementRepository defines the data access methods for DPAs.
type DataProcessingAgreementRepository interface {
	Create(ctx context.Context, agreement *models.DataProcessingAgreement) error
	FindByID(ctx context.Context, id uint) (*models.DataProcessingAgreement, error)
	Update(ctx context.Context, agreement *models.DataProcessingAgreement) error
	ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DataProcessingAgreement], error)
	ListActive(ctx context.Context, accountID uint) ([]models.DataProcessingAgreement, error)
}

type dataProcessingAgreementRepo struct {
	db *gorm.DB
}

// NewDataProcessingAgreementRepository creates a new DataProcessingAgreement repository backed by PostgreSQL.
func NewDataProcessingAgreementRepository(db *gorm.DB) DataProcessingAgreementRepository {
	return &dataProcessingAgreementRepo{db: db}
}

func (r *dataProcessingAgreementRepo) Create(ctx context.Context, agreement *models.DataProcessingAgreement) error {
	return r.db.WithContext(ctx).Create(agreement).Error
}

func (r *dataProcessingAgreementRepo) FindByID(ctx context.Context, id uint) (*models.DataProcessingAgreement, error) {
	var agreement models.DataProcessingAgreement
	if err := r.db.WithContext(ctx).First(&agreement, id).Error; err != nil {
		return nil, err
	}
	return &agreement, nil
}

func (r *dataProcessingAgreementRepo) Update(ctx context.Context, agreement *models.DataProcessingAgreement) error {
	return r.db.WithContext(ctx).Save(agreement).Error
}

func (r *dataProcessingAgreementRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DataProcessingAgreement], error) {
	var agreements []models.DataProcessingAgreement
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DataProcessingAgreement{}).Where("account_id = ?", accountID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&agreements).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DataProcessingAgreement]{
		Items:      agreements,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *dataProcessingAgreementRepo) ListActive(ctx context.Context, accountID uint) ([]models.DataProcessingAgreement, error) {
	var agreements []models.DataProcessingAgreement
	if err := r.db.WithContext(ctx).Where("account_id = ? AND status = ?", accountID, "active").Order("created_at DESC").Find(&agreements).Error; err != nil {
		return nil, err
	}
	return agreements, nil
}

// AgeVerificationRepository defines the data access methods for age verification records.
type AgeVerificationRepository interface {
	Create(ctx context.Context, verification *models.AgeVerification) error
	FindByUserID(ctx context.Context, userID uint) (*models.AgeVerification, error)
	Update(ctx context.Context, verification *models.AgeVerification) error
}

type ageVerificationRepo struct {
	db *gorm.DB
}

// NewAgeVerificationRepository creates a new AgeVerification repository backed by PostgreSQL.
func NewAgeVerificationRepository(db *gorm.DB) AgeVerificationRepository {
	return &ageVerificationRepo{db: db}
}

func (r *ageVerificationRepo) Create(ctx context.Context, verification *models.AgeVerification) error {
	return r.db.WithContext(ctx).Create(verification).Error
}

func (r *ageVerificationRepo) FindByUserID(ctx context.Context, userID uint) (*models.AgeVerification, error) {
	var verification models.AgeVerification
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&verification).Error; err != nil {
		return nil, err
	}
	return &verification, nil
}

func (r *ageVerificationRepo) Update(ctx context.Context, verification *models.AgeVerification) error {
	return r.db.WithContext(ctx).Save(verification).Error
}
