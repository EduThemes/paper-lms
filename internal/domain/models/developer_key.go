package models

import "time"

type DeveloperKey struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID     uint      `json:"account_id" gorm:"default:1"`
	Name          string    `json:"name" gorm:"not null"`
	Email         string    `json:"email"`
	ClientID      string    `json:"api_key" gorm:"uniqueIndex;not null"` // Canvas calls this api_key
	ClientSecret  string    `json:"-" gorm:"not null"`                   // Never exposed in API
	RedirectURI   string    `json:"redirect_uri"`
	RedirectURIs  string    `json:"redirect_uris" gorm:"type:text"` // Newline-separated list
	Icon          string    `json:"icon_url"`
	Notes         string    `json:"notes" gorm:"type:text"`
	Scopes        string    `json:"scopes" gorm:"type:text"` // JSON array of scope strings
	RequireScopes bool      `json:"require_scopes" gorm:"default:false"`
	WorkflowState string    `json:"workflow_state" gorm:"default:'active'"` // active, inactive, deleted
	IsLTIKey      bool      `json:"is_lti_key" gorm:"default:false"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
