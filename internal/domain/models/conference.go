package models

import "time"

type Conference struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	ContextType    string     `json:"context_type" gorm:"not null;index:idx_conf_context"` // Course, Group
	ContextID      uint       `json:"context_id" gorm:"not null;index:idx_conf_context"`
	ConferenceType string     `json:"conference_type" gorm:"not null"` // BigBlueButton, Zoom
	Title          string     `json:"title" gorm:"not null"`
	Description    string     `json:"description" gorm:"type:text"`
	UserID         uint       `json:"user_id" gorm:"not null"`
	StartedAt      *time.Time `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at"`
	Duration       int        `json:"duration" gorm:"default:0"`
	JoinURL        string     `json:"join_url"`
	Recordings     string     `json:"recordings" gorm:"type:jsonb;default:'[]'"`
	Settings       string     `json:"settings" gorm:"type:jsonb;default:'{}'"`
	WorkflowState  string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
