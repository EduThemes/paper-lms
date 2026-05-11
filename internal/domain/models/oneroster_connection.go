package models

import "time"

type OneRosterConnection struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	AccountID        uint       `json:"account_id" gorm:"not null;index"`
	Name             string     `json:"name" gorm:"not null"`
	BaseURL          string     `json:"base_url" gorm:"not null"`
	ClientID         string     `json:"client_id" gorm:"not null"`
	ClientSecret     string     `json:"-" gorm:"not null"` // encrypted at rest, never serialized
	TokenURL         string     `json:"token_url" gorm:"not null"`
	Scope            string     `json:"scope" gorm:"default:'https://purl.imsglobal.org/spec/or/v1p1/scope/roster-core.readonly'"`
	LastSyncAt       *time.Time `json:"last_sync_at"`
	SyncStatus       string     `json:"sync_status" gorm:"not null;default:'idle'"` // idle, syncing, error, completed
	LastSyncError    string     `json:"last_sync_error" gorm:"type:text"`
	SyncFilter       string     `json:"sync_filter" gorm:"type:text"` // JSON: which orgs, terms to include
	AutoSync         bool       `json:"auto_sync" gorm:"default:false"`
	AutoSyncInterval int        `json:"auto_sync_interval" gorm:"default:24"` // hours
	WorkflowState    string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
