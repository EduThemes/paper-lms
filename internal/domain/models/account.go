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
	MaxUploadSizeMB uint      `json:"max_upload_size_mb" gorm:"not null;default:500"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
