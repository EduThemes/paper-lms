package models

import "time"

type LatePolicy struct {
	ID                                  uint      `json:"id" gorm:"primaryKey"`
	CourseID                            uint      `json:"course_id" gorm:"not null;uniqueIndex"`
	MissingSubmissionDeductionEnabled   bool      `json:"missing_submission_deduction_enabled" gorm:"default:false"`
	MissingSubmissionDeduction          float64   `json:"missing_submission_deduction" gorm:"default:0"`
	LateSubmissionDeductionEnabled      bool      `json:"late_submission_deduction_enabled" gorm:"default:false"`
	LateSubmissionDeduction             float64   `json:"late_submission_deduction" gorm:"default:0"`
	LateSubmissionInterval              string    `json:"late_submission_interval" gorm:"default:'day'"` // day, hour
	LateSubmissionMinimumPercentEnabled bool      `json:"late_submission_minimum_percent_enabled" gorm:"default:false"`
	LateSubmissionMinimumPercent        float64   `json:"late_submission_minimum_percent" gorm:"default:0"`
	CreatedAt                           time.Time `json:"created_at"`
	UpdatedAt                           time.Time `json:"updated_at"`
}
