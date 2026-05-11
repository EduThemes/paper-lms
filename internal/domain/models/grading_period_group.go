package models

import "time"

type GradingPeriodGroup struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	AccountID     uint      `json:"account_id" gorm:"not null;index"`
	Title         string    `json:"title" gorm:"not null"`
	Weighted      bool      `json:"weighted" gorm:"default:false"`
	DisplayTotals bool      `json:"display_totals_for_all_grading_periods" gorm:"default:false"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
