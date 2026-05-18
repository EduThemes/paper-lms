package models

import "time"

type LTIResult struct {
	ID               uint       `json:"id" gorm:"column:id;primaryKey"`
	LineItemID       uint       `json:"line_item_id" gorm:"not null;index"`
	UserID           uint       `json:"userId" gorm:"not null;index"`
	ResultScore      *float64   `json:"resultScore"`
	ResultMaximum    *float64   `json:"resultMaximum"`
	Comment          string     `json:"comment"`
	ActivityProgress string     `json:"activityProgress" gorm:"default:'Initialized'"` // Initialized, Started, InProgress, Submitted, Completed
	GradingProgress  string     `json:"gradingProgress" gorm:"default:'NotReady'"`     // FullyGraded, Pending, PendingManual, Failed, NotReady
	Timestamp        *time.Time `json:"timestamp"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// Composite unique: line_item_id + user_id
}

func (LTIResult) TableName() string { return "lti_results" }
