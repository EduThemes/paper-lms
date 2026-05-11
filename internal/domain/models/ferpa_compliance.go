package models

import "time"

// DataRetentionPolicy defines how long different categories of student data are retained.
type DataRetentionPolicy struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	AccountID       uint      `json:"account_id" gorm:"not null;index"`
	DataCategory    string    `json:"data_category" gorm:"not null"` // student_records, submissions, grades, messages, files, logs
	RetentionPeriod int       `json:"retention_period"`              // days
	RetentionAction string    `json:"retention_action" gorm:"not null;default:'anonymize'"` // delete, anonymize, archive
	AutoApply       bool      `json:"auto_apply" gorm:"default:false"`
	Description     string    `json:"description" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DataDeletionRequest tracks requests to delete or anonymize student data per FERPA.
type DataDeletionRequest struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	RequestedByID uint       `json:"requested_by_id" gorm:"not null;index"`
	UserID        uint       `json:"user_id" gorm:"not null;index"` // student whose data to delete
	RequestType   string     `json:"request_type" gorm:"not null"`  // full_deletion, selective_deletion, anonymization
	DataScope     string     `json:"data_scope" gorm:"type:text"`   // JSON: which data categories to delete
	Reason        string     `json:"reason" gorm:"type:text"`
	Status        string     `json:"status" gorm:"not null;default:'pending'"` // pending, approved, processing, completed, denied
	ReviewedByID  *uint      `json:"reviewed_by_id"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	DeletionLog   string     `json:"deletion_log" gorm:"type:text"` // JSON log of what was deleted
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// DataExportRequest tracks requests to export student data (FERPA right to access).
type DataExportRequest struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	RequestedByID uint       `json:"requested_by_id" gorm:"not null;index"`
	UserID        uint       `json:"user_id" gorm:"not null;index"`            // student whose data to export
	ExportFormat  string     `json:"export_format" gorm:"not null;default:'json'"` // json, csv, zip
	DataScope     string     `json:"data_scope" gorm:"type:text"`              // JSON: which categories to include
	Status        string     `json:"status" gorm:"not null;default:'pending'"` // pending, processing, completed, failed, expired
	DownloadURL   string     `json:"download_url"`
	ExpiresAt     *time.Time `json:"expires_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	FileSizeBytes int64      `json:"file_size_bytes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// PIIAccessLog records every access to personally identifiable information for FERPA audit trail.
type PIIAccessLog struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	AccessorID    uint      `json:"accessor_id" gorm:"not null;index"`
	StudentID     uint      `json:"student_id" gorm:"not null;index"`
	AccessType    string    `json:"access_type" gorm:"not null"` // view, export, modify, delete
	DataField     string    `json:"data_field" gorm:"not null"`  // which PII field was accessed
	Resource      string    `json:"resource"`                    // e.g., "user_profile", "grade_record", "submission"
	ResourceID    uint      `json:"resource_id"`
	IPAddress     string    `json:"ip_address"`
	UserAgent     string    `json:"user_agent"`
	Justification string   `json:"justification" gorm:"type:text"`
	CreatedAt     time.Time `json:"created_at"`
}
