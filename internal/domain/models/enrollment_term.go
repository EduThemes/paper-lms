package models

import "time"

type EnrollmentTerm struct {
	ID                   uint       `json:"id" gorm:"column:id;primaryKey"`
	AccountID uint   `json:"account_id" gorm:"index;uniqueIndex:idx_enrollment_terms_sis_term_id,priority:1,where:sis_term_id IS NOT NULL"`
	Name      string `json:"name"`
	// SISTermID is nullable under a partial UNIQUE (account_id,
	// sis_term_id) index (migration 000071), so it MUST be *string: a
	// blank SIS id is stored as NULL, never '', otherwise the second
	// term created without one collides on the index (B071).
	SISTermID            *string    `json:"sis_term_id" gorm:"column:sis_term_id;uniqueIndex:idx_enrollment_terms_sis_term_id,priority:2,where:sis_term_id IS NOT NULL"`
	StartAt              *time.Time `json:"start_at"`
	EndAt                *time.Time `json:"end_at"`
	GradingPeriodGroupID *uint      `json:"grading_period_group_id"`
	WorkflowState        string     `json:"workflow_state" gorm:"default:active"` // active, deleted
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
