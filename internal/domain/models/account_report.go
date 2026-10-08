package models

import (
	"time"

	"gorm.io/gorm"
)

// AccountReportState is the lifecycle of an AccountReport run, using
// Canvas's report status values.
type AccountReportState string

const (
	AccountReportCreated  AccountReportState = "created"
	AccountReportRunning  AccountReportState = "running"
	AccountReportComplete AccountReportState = "complete"
	AccountReportError    AccountReportState = "error"
	AccountReportAborted  AccountReportState = "aborted"
	AccountReportDeleted  AccountReportState = "deleted"
)

// AccountReport is one run of a Canvas-style account report (G138:
// /api/v1/accounts/:account_id/reports). The generated CSV (or ZIP of
// CSVs) is written to the storage backend under FileKey. Migration 000159.
type AccountReport struct {
	ID        uint `json:"id" gorm:"column:id;primaryKey"`
	AccountID uint `json:"account_id" gorm:"not null;index"`
	// UserID is the admin who started the run.
	UserID     uint   `json:"user_id" gorm:"not null;index"`
	ReportType string `json:"report" gorm:"column:report_type;type:text;not null;index"`
	// WorkflowState is NOT NULL with a SQL DEFAULT and a CHECK; BeforeCreate
	// mirrors the default because GORM writes '' for a zero string.
	WorkflowState AccountReportState `json:"status" gorm:"type:text;not null"`
	// Parameters is the JSON object of report parameters the run used.
	Parameters  string     `json:"-" gorm:"type:text;not null"`
	Progress    int        `json:"progress" gorm:"not null"`
	Message     string     `json:"message" gorm:"type:text"`
	FileKey     *string    `json:"-" gorm:"column:file_key;type:text"`
	FileName    *string    `json:"-" gorm:"column:file_name;type:text"`
	ContentType *string    `json:"-" gorm:"column:content_type;type:text"`
	FileSize    int64      `json:"-" gorm:"column:file_size;not null"`
	StartedAt   *time.Time `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName pins the table name.
func (AccountReport) TableName() string { return "account_reports" }

// BeforeCreate mirrors the SQL DEFAULTs of migration 000159.
func (r *AccountReport) BeforeCreate(tx *gorm.DB) error {
	if r.WorkflowState == "" {
		r.WorkflowState = AccountReportCreated
	}
	if r.Parameters == "" {
		r.Parameters = "{}"
	}
	return nil
}
