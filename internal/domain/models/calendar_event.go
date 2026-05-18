package models

import "time"

type CalendarEvent struct {
	ID              uint       `json:"id" gorm:"column:id;primaryKey"`
	ContextType     string     `json:"context_type" gorm:"not null;index:idx_cal_event_context"`
	ContextID       uint       `json:"context_id" gorm:"not null;index:idx_cal_event_context"`
	Title           string     `json:"title" gorm:"not null"`
	Description     string     `json:"description" gorm:"type:text"`
	StartAt         time.Time  `json:"start_at" gorm:"not null;index"`
	EndAt           *time.Time `json:"end_at"`
	LocationName    string     `json:"location_name"`
	LocationAddress string     `json:"location_address"`
	AllDay          bool       `json:"all_day" gorm:"default:false"`
	CreatedByUserID uint       `json:"created_by_user_id" gorm:"not null"`
	WorkflowState   string     `json:"workflow_state" gorm:"default:'active'"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
