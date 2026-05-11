package models

import "time"

type OneRosterSyncLog struct {
	ID                   uint       `json:"id" gorm:"primaryKey"`
	ConnectionID         uint       `json:"connection_id" gorm:"not null;index"`
	SyncType             string     `json:"sync_type" gorm:"not null"` // full, incremental
	Status               string     `json:"status" gorm:"not null"`   // running, completed, failed
	OrgsCreated          int        `json:"orgs_created" gorm:"default:0"`
	OrgsUpdated          int        `json:"orgs_updated" gorm:"default:0"`
	UsersCreated         int        `json:"users_created" gorm:"default:0"`
	UsersUpdated         int        `json:"users_updated" gorm:"default:0"`
	ClassesCreated       int        `json:"classes_created" gorm:"default:0"`
	ClassesUpdated       int        `json:"classes_updated" gorm:"default:0"`
	EnrollmentsCreated   int        `json:"enrollments_created" gorm:"default:0"`
	EnrollmentsUpdated   int        `json:"enrollments_updated" gorm:"default:0"`
	Errors               int        `json:"errors" gorm:"default:0"`
	StartedAt            *time.Time `json:"started_at"`
	CompletedAt          *time.Time `json:"completed_at"`
	ErrorDetails         string     `json:"error_details" gorm:"type:text"` // JSON array of error strings
}
