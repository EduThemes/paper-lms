package models

import "time"

type GradingPeriod struct {
	ID                   uint       `json:"id" gorm:"primaryKey"`
	GradingPeriodGroupID uint       `json:"grading_period_group_id" gorm:"not null;index"`
	Title                string     `json:"title" gorm:"not null"`
	StartDate            time.Time  `json:"start_date" gorm:"not null"`
	EndDate              time.Time  `json:"end_date" gorm:"not null"`
	CloseDate            *time.Time `json:"close_date"` // when grades lock
	Weight               *float64   `json:"weight"`
	IsClosed             bool       `json:"is_closed" gorm:"default:false"`
	WorkflowState        string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
