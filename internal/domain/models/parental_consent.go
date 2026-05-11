package models

import "time"

// ParentalConsent tracks COPPA 2025 parental consent for students under 13.
type ParentalConsent struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	StudentID         uint       `json:"student_id" gorm:"not null;index"`
	ParentUserID      *uint      `json:"parent_user_id" gorm:"index"`
	ParentName        string     `json:"parent_name" gorm:"not null"`
	ParentEmail       string     `json:"parent_email" gorm:"not null"`
	ConsentType       string     `json:"consent_type" gorm:"not null"`                    // data_collection, third_party_sharing, marketing
	Status            string     `json:"status" gorm:"not null;default:'pending'"`         // pending, granted, denied, revoked
	ConsentMethod     string     `json:"consent_method"`                                   // email_verification, signed_form, in_person, sso_verified
	VerificationToken string     `json:"-" gorm:"index"`
	ConsentedAt       *time.Time `json:"consented_at"`
	RevokedAt         *time.Time `json:"revoked_at"`
	ExpiresAt         *time.Time `json:"expires_at"`
	IPAddress         string     `json:"-"`
	UserAgent         string     `json:"-"`
	Notes             string     `json:"notes" gorm:"type:text"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DataProcessingAgreement tracks vendor data processing agreements for COPPA compliance.
type DataProcessingAgreement struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	AccountID       uint       `json:"account_id" gorm:"not null;index"`
	VendorName      string     `json:"vendor_name" gorm:"not null"`
	VendorContact   string     `json:"vendor_contact"`
	Purpose         string     `json:"purpose" gorm:"type:text;not null"`
	DataCategories  string     `json:"data_categories" gorm:"type:text"` // JSON array of data types collected
	RetentionPeriod string     `json:"retention_period"`                 // e.g., "end_of_school_year", "3_years", "until_graduation"
	Status          string     `json:"status" gorm:"not null;default:'draft'"` // draft, active, expired, terminated
	SignedAt        *time.Time `json:"signed_at"`
	ExpiresAt       *time.Time `json:"expires_at"`
	DocumentURL     string     `json:"document_url"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AgeVerification stores age verification data for COPPA compliance checks.
type AgeVerification struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	UserID          uint       `json:"user_id" gorm:"not null;uniqueIndex"`
	DateOfBirth     *time.Time `json:"date_of_birth"`
	IsUnder13       bool       `json:"is_under_13" gorm:"not null;default:false"`
	IsMinor         bool       `json:"is_minor" gorm:"not null;default:true"`
	VerifiedBy      string     `json:"verified_by"` // sis_import, parent_attestation, self_reported, admin
	RequiresConsent bool       `json:"requires_consent" gorm:"not null;default:true"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
