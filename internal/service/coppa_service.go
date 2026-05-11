package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// COPPAService provides business logic for COPPA 2025 compliance.
type COPPAService struct {
	consentRepo      postgres.ParentalConsentRepository
	dpaRepo          postgres.DataProcessingAgreementRepository
	ageVerifyRepo    postgres.AgeVerificationRepository
}

// NewCOPPAService creates a new COPPAService with the given repository dependencies.
func NewCOPPAService(
	consentRepo postgres.ParentalConsentRepository,
	dpaRepo postgres.DataProcessingAgreementRepository,
	ageVerifyRepo postgres.AgeVerificationRepository,
) *COPPAService {
	return &COPPAService{
		consentRepo:   consentRepo,
		dpaRepo:       dpaRepo,
		ageVerifyRepo: ageVerifyRepo,
	}
}

// VerifyConsentRequired checks age verification for a user and returns whether parental consent is needed.
func (s *COPPAService) VerifyConsentRequired(ctx context.Context, userID uint) (bool, error) {
	verification, err := s.ageVerifyRepo.FindByUserID(ctx, userID)
	if err != nil {
		// No age verification record found; assume consent is required for safety
		return true, nil
	}
	return verification.RequiresConsent, nil
}

// RequestParentalConsent creates a new parental consent record and generates a verification token.
func (s *COPPAService) RequestParentalConsent(ctx context.Context, studentID uint, parentEmail string, consentType string) (*models.ParentalConsent, error) {
	if parentEmail == "" {
		return nil, errors.New("parent email is required")
	}
	if consentType == "" {
		return nil, errors.New("consent type is required")
	}

	// Validate consent type
	validTypes := map[string]bool{
		"data_collection":      true,
		"third_party_sharing":  true,
		"marketing":            true,
	}
	if !validTypes[consentType] {
		return nil, errors.New("consent_type must be one of: data_collection, third_party_sharing, marketing")
	}

	// Generate a secure verification token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errors.New("failed to generate verification token")
	}
	token := hex.EncodeToString(tokenBytes)

	consent := &models.ParentalConsent{
		StudentID:         studentID,
		ParentEmail:       parentEmail,
		ParentName:        "",
		ConsentType:       consentType,
		Status:            "pending",
		ConsentMethod:     "email_verification",
		VerificationToken: token,
	}

	if err := s.consentRepo.Create(ctx, consent); err != nil {
		return nil, err
	}

	return consent, nil
}

// VerifyConsent validates a verification token and updates the consent status to granted.
func (s *COPPAService) VerifyConsent(ctx context.Context, token string) (*models.ParentalConsent, error) {
	if token == "" {
		return nil, errors.New("verification token is required")
	}

	consent, err := s.consentRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, errors.New("invalid or expired verification token")
	}

	if consent.Status != "pending" {
		return nil, errors.New("consent has already been processed")
	}

	// Check if the consent has expired
	if consent.ExpiresAt != nil && consent.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("verification token has expired")
	}

	now := time.Now()
	consent.Status = "granted"
	consent.ConsentedAt = &now
	consent.VerificationToken = "" // Clear token after use

	if err := s.consentRepo.Update(ctx, consent); err != nil {
		return nil, err
	}

	return consent, nil
}

// RevokeConsent revokes a previously granted consent and records the revocation timestamp.
func (s *COPPAService) RevokeConsent(ctx context.Context, consentID uint, revokerID uint) error {
	consent, err := s.consentRepo.FindByID(ctx, consentID)
	if err != nil {
		return errors.New("consent record not found")
	}

	if consent.Status == "revoked" {
		return errors.New("consent has already been revoked")
	}

	now := time.Now()
	consent.Status = "revoked"
	consent.RevokedAt = &now

	return s.consentRepo.Update(ctx, consent)
}

// CheckConsentStatus returns whether consent is granted for a specific student and consent type.
func (s *COPPAService) CheckConsentStatus(ctx context.Context, studentID uint, consentType string) (bool, error) {
	consents, err := s.consentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return false, err
	}

	for _, consent := range consents {
		if consent.ConsentType == consentType && consent.Status == "granted" {
			// Check if the consent has expired
			if consent.ExpiresAt != nil && consent.ExpiresAt.Before(time.Now()) {
				continue
			}
			return true, nil
		}
	}

	return false, nil
}

// ListConsentsForStudent returns all consent records for a given student.
func (s *COPPAService) ListConsentsForStudent(ctx context.Context, studentID uint) ([]models.ParentalConsent, error) {
	return s.consentRepo.FindByStudentID(ctx, studentID)
}

// CreateDPA creates a new data processing agreement.
func (s *COPPAService) CreateDPA(ctx context.Context, agreement *models.DataProcessingAgreement) error {
	if agreement.VendorName == "" {
		return errors.New("vendor name is required")
	}
	if agreement.Purpose == "" {
		return errors.New("purpose is required")
	}
	if agreement.AccountID == 0 {
		return errors.New("account_id is required")
	}
	if agreement.Status == "" {
		agreement.Status = "draft"
	}

	return s.dpaRepo.Create(ctx, agreement)
}

// UpdateDPA updates an existing data processing agreement.
func (s *COPPAService) UpdateDPA(ctx context.Context, agreement *models.DataProcessingAgreement) error {
	_, err := s.dpaRepo.FindByID(ctx, agreement.ID)
	if err != nil {
		return errors.New("data processing agreement not found")
	}

	return s.dpaRepo.Update(ctx, agreement)
}

// GetDPA retrieves a data processing agreement by ID.
func (s *COPPAService) GetDPA(ctx context.Context, id uint) (*models.DataProcessingAgreement, error) {
	dpa, err := s.dpaRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("data processing agreement not found")
	}
	return dpa, nil
}

// ListActiveDPAs returns all active data processing agreements for an account.
func (s *COPPAService) ListActiveDPAs(ctx context.Context, accountID uint) ([]models.DataProcessingAgreement, error) {
	return s.dpaRepo.ListActive(ctx, accountID)
}

// ListDPAs returns a paginated list of all data processing agreements for an account.
func (s *COPPAService) ListDPAs(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DataProcessingAgreement], error) {
	return s.dpaRepo.ListByAccountID(ctx, accountID, params)
}

// CreateAgeVerification creates or updates an age verification record for a user.
func (s *COPPAService) CreateAgeVerification(ctx context.Context, verification *models.AgeVerification) error {
	if verification.UserID == 0 {
		return errors.New("user_id is required")
	}

	// Check if the user is under 13 based on date of birth
	if verification.DateOfBirth != nil {
		age := calculateAge(*verification.DateOfBirth)
		verification.IsUnder13 = age < 13
		verification.IsMinor = age < 18
		verification.RequiresConsent = age < 13
	}

	return s.ageVerifyRepo.Create(ctx, verification)
}

// GetAgeVerification retrieves the age verification record for a user.
func (s *COPPAService) GetAgeVerification(ctx context.Context, userID uint) (*models.AgeVerification, error) {
	return s.ageVerifyRepo.FindByUserID(ctx, userID)
}

// calculateAge computes the age in years from a date of birth.
func calculateAge(dob time.Time) int {
	now := time.Now()
	age := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		age--
	}
	return age
}
