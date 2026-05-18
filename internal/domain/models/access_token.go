package models

import "time"

type AccessToken struct {
	ID             uint       `json:"id" gorm:"column:id;primaryKey"`
	UserID         uint       `json:"user_id" gorm:"not null;index"`
	DeveloperKeyID *uint      `json:"developer_key_id" gorm:"index"` // nil for personal access tokens
	Token          string     `json:"-" gorm:"uniqueIndex;not null"` // Hashed token, never expose
	TokenHint      string     `json:"token_hint"`                    // Last 4 chars for display
	RefreshToken   *string    `json:"-" gorm:"uniqueIndex"`          // For OAuth2 tokens (nil for PATs)
	Scopes         string     `json:"scopes" gorm:"type:text"`       // JSON array
	Purpose        string     `json:"purpose"`                       // User-provided description for PATs
	ExpiresAt      *time.Time `json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	WorkflowState  string     `json:"workflow_state" gorm:"default:'active'"` // active, deleted
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	// Associations
	User         User          `json:"-" gorm:"foreignKey:UserID"`
	DeveloperKey *DeveloperKey `json:"-" gorm:"foreignKey:DeveloperKeyID"`
}
