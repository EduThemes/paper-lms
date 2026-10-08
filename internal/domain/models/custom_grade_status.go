package models

import (
	"time"

	"gorm.io/gorm"
)

// CustomGradeStatus is an account-defined grade status (Canvas
// custom_grade_statuses): a named, coloured label a grader can put on a
// submission (and, when AppliesToFinalGrade, on a course total) in
// addition to the built-in late / missing / excused / extended.
type CustomGradeStatus struct {
	ID                   uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID            uint      `json:"account_id" gorm:"not null;index"`
	Name                 string    `json:"name" gorm:"type:text;not null"`
	Color                string    `json:"color" gorm:"type:text;not null;default:'#E0E0E0'"`
	AppliesToSubmissions bool      `json:"applies_to_submissions" gorm:"not null;default:false"`
	AppliesToFinalGrade  bool      `json:"applies_to_final_grade" gorm:"not null;default:false"`
	WorkflowState        string    `json:"workflow_state" gorm:"type:text;not null;default:'active';index"`
	CreatedByID          *uint     `json:"created_by_id"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// MaxCustomGradeStatuses is Canvas's per-account limit on active
// custom statuses.
const MaxCustomGradeStatuses = 3

// BeforeCreate mirrors the SQL NOT NULL DEFAULTs: GORM writes the empty
// string instead of omitting the column.
func (s *CustomGradeStatus) BeforeCreate(tx *gorm.DB) error {
	if s.Color == "" {
		s.Color = "#E0E0E0"
	}
	if s.WorkflowState == "" {
		s.WorkflowState = "active"
	}
	return nil
}
