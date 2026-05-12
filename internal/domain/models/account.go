package models

import "time"

type Account struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Name            string    `json:"name" gorm:"not null"`
	ParentAccountID *uint     `json:"parent_account_id"`
	RootAccountID   *uint     `json:"root_account_id"`
	SISAccountID    *string   `json:"sis_account_id" gorm:"uniqueIndex"`
	WorkflowState   string    `json:"workflow_state" gorm:"not null;default:'active'"`
	// MaxUploadSizeMB caps file upload size (per request) for this account.
	// Default 500. Editable by admins via the settings page; enforced by middleware.
	MaxUploadSizeMB uint `json:"max_upload_size_mb" gorm:"not null;default:500"`
	// TenantMode drives every gamification + compliance default. Phase 6 Wave 1
	// (migration 000035) added the column with default 'higher_ed'. K-12 tenants
	// migrate manually. Values mirror the gamification_audience enum:
	// k5 | m68 | h912 | higher_ed | corp | pro. Field declared as plain string so
	// AutoMigrate doesn't need the enum type to exist at parity-test time.
	TenantMode GamificationAudience `json:"tenant_mode" gorm:"not null;type:text;default:'higher_ed'"`
	// CoppaStrict force-applies COPPA defaults regardless of TenantMode. Used for
	// K-12 deployments handling under-13 users.
	CoppaStrict bool      `json:"coppa_strict" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
