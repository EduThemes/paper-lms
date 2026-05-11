package models

import (
	"time"

	"gorm.io/datatypes"
)

// MigrationSettings is the typed shape stored in ContentMigration.MigrationSettings.
// Fields are intentionally narrow — only what the current import pipeline reads/writes.
// New fields can be added without a SQL migration since the column is jsonb.
type MigrationSettings struct {
	SourceCourseID     *uint             `json:"source_course_id,omitempty"`
	OverwriteQuizzes   bool              `json:"overwrite_quizzes,omitempty"`
	InsertIntoModuleID *uint             `json:"insert_into_module_id,omitempty"`
	DateShiftOptions   *DateShiftOptions `json:"date_shift_options,omitempty"`
	CopyEverything     bool              `json:"copy_everything,omitempty"`
	IncludeOnlyTypes   []string          `json:"include_only_types,omitempty"`
	ExcludeTypes       []string          `json:"exclude_types,omitempty"`
	// LegacyString preserves any pre-typed text payloads carried over by the
	// 000003 migration so older rows don't lose information.
	LegacyString string `json:"legacy_string,omitempty"`
}

// DateShiftOptions describes how Canvas-style date-shift import options are encoded.
type DateShiftOptions struct {
	ShiftDates       bool              `json:"shift_dates,omitempty"`
	OldStartDate     *time.Time        `json:"old_start_date,omitempty"`
	NewStartDate     *time.Time        `json:"new_start_date,omitempty"`
	OldEndDate       *time.Time        `json:"old_end_date,omitempty"`
	NewEndDate       *time.Time        `json:"new_end_date,omitempty"`
	DaySubstitutions map[string]string `json:"day_substitutions,omitempty"`
	RemoveDates      bool              `json:"remove_dates,omitempty"`
}

// ContentMigration tracks the import of content packages (Common Cartridge, IMSCC, Canvas export).
type ContentMigration struct {
	ID                uint                                  `json:"id" gorm:"primaryKey"`
	CourseID          uint                                  `json:"course_id" gorm:"not null;index"`
	UserID            uint                                  `json:"user_id" gorm:"not null"`
	MigrationType     string                                `json:"migration_type" gorm:"not null"`                   // common_cartridge, canvas_cartridge, course_copy, qti_converter, zip_file_importer
	SourceCourseID    *uint                                 `json:"source_course_id"`                                 // For course_copy
	WorkflowState     string                                `json:"workflow_state" gorm:"not null;default:'created'"` // created, pre_processing, running, completed, failed
	Progress          int                                   `json:"progress" gorm:"default:0"`                        // 0-100
	MigrationSettings datatypes.JSONType[MigrationSettings] `json:"migration_settings" gorm:"type:jsonb"`             // typed JSON: selective import options
	StartedAt         *time.Time                            `json:"started_at"`
	FinishedAt        *time.Time                            `json:"finished_at"`
	ErrorMessage      string                                `json:"error_message" gorm:"type:text"`
	Attachment        string                                `json:"attachment" gorm:"type:text"` // path to uploaded package file
	CreatedAt         time.Time                             `json:"created_at"`
	UpdatedAt         time.Time                             `json:"updated_at"`
}
