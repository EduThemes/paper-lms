package models

import (
	"time"

	"gorm.io/gorm"
)

// AccountReport is one run of an account-level report (G117; Canvas:
// AccountReport). The generated CSV is stored on the row so it can be
// downloaded later without a file store.
type AccountReport struct {
	ID            uint       `json:"id" gorm:"column:id;primaryKey"`
	AccountID     uint       `json:"account_id" gorm:"not null;index"`
	UserID        uint       `json:"user_id" gorm:"not null;index"` // who ran it
	ReportType    string     `json:"report" gorm:"column:report_type;not null;index"`
	Parameters    string     `json:"parameters" gorm:"type:jsonb;not null;default:'{}'"`
	WorkflowState string     `json:"status" gorm:"column:workflow_state;not null;default:'created'"` // created, running, complete, error, aborted
	Progress      int        `json:"progress" gorm:"not null;default:0"`
	Message       string     `json:"message" gorm:"type:text;not null;default:''"`
	RowCount      int        `json:"row_count" gorm:"not null;default:0"`
	FileName      string     `json:"file_name" gorm:"not null;default:''"`
	FileData      []byte     `json:"-" gorm:"type:bytea"`
	StartedAt     *time.Time `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AccountReport workflow states.
const (
	AccountReportCreated  = "created"
	AccountReportRunning  = "running"
	AccountReportComplete = "complete"
	AccountReportError    = "error"
	AccountReportAborted  = "aborted"
)

// BeforeCreate mirrors the SQL DEFAULTs (GORM writes an empty string for
// zero strings, and an empty string is not valid jsonb).
func (r *AccountReport) BeforeCreate(tx *gorm.DB) error {
	if r.Parameters == "" {
		r.Parameters = "{}"
	}
	if r.WorkflowState == "" {
		r.WorkflowState = AccountReportCreated
	}
	return nil
}
