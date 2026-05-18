package models

import "time"

type EnrollmentTerm struct {
	ID                   uint       `json:"id" gorm:"column:id;primaryKey"`
	AccountID            uint       `json:"account_id" gorm:"index"`
	Name                 string     `json:"name"`
	SISTermID            string     `json:"sis_term_id" gorm:"column:sis_term_id;uniqueIndex"`
	StartAt              *time.Time `json:"start_at"`
	EndAt                *time.Time `json:"end_at"`
	GradingPeriodGroupID *uint      `json:"grading_period_group_id"`
	WorkflowState        string     `json:"workflow_state" gorm:"default:active"` // active, deleted
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
