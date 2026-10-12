package models

import (
	"time"

	"gorm.io/gorm"
)

// ContentExportWorkflow tracks the lifecycle of a ContentExport job.
// Values follow Canvas's content_exports API: created → exporting →
// exported | failed.
type ContentExportWorkflow string

const (
	ContentExportCreated   ContentExportWorkflow = "created"
	ContentExportExporting ContentExportWorkflow = "exporting"
	ContentExportExported  ContentExportWorkflow = "exported"
	ContentExportFailed    ContentExportWorkflow = "failed"
)

// ContentExportTypeCommonCartridge is the only export type Paper
// produces today: an IMS Common Cartridge (.imscc) of the course.
const ContentExportTypeCommonCartridge = "common_cartridge"

// ContentExport is one course content export (Canvas ContentExport):
// the request, its progress, and — once exported — the storage key of the
// generated package.
type ContentExport struct {
	ID            uint                  `json:"id" gorm:"column:id;primaryKey"`
	CourseID      uint                  `json:"course_id" gorm:"not null;index"`
	UserID        uint                  `json:"user_id" gorm:"not null"`
	ExportType    string                `json:"export_type" gorm:"type:text;not null;default:'common_cartridge'"`
	WorkflowState ContentExportWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'created'"`
	Progress      int                   `json:"progress" gorm:"not null;default:0"`
	// Selection is the JSON-encoded ContentExportSelection ("{}" = whole
	// course).
	Selection    string     `json:"selection" gorm:"type:jsonb;not null;default:'{}'"`
	StorageKey   string     `json:"-" gorm:"type:text"`
	FileName     string     `json:"file_name" gorm:"type:text"`
	FileSize     int64      `json:"file_size" gorm:"not null;default:0"`
	Results      string     `json:"results" gorm:"type:jsonb;not null;default:'{}'"`
	ErrorMessage string     `json:"error_message" gorm:"type:text"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (ContentExport) TableName() string { return "content_exports" }

// BeforeCreate mirrors the NOT NULL DEFAULTs of migration 000097: GORM
// writes an empty string for a zero string, defeating the SQL default.
func (e *ContentExport) BeforeCreate(tx *gorm.DB) error {
	if e.ExportType == "" {
		e.ExportType = ContentExportTypeCommonCartridge
	}
	if e.WorkflowState == "" {
		e.WorkflowState = ContentExportCreated
	}
	if e.Selection == "" {
		e.Selection = "{}"
	}
	if e.Results == "" {
		e.Results = "{}"
	}
	return nil
}
