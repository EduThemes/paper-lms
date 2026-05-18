package models

import "time"

type AssignmentOverrideStudent struct {
	ID                   uint      `json:"id" gorm:"column:id;primaryKey"`
	AssignmentOverrideID uint      `json:"assignment_override_id" gorm:"not null;index"`
	UserID               uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_override_user"`
	AssignmentID         uint      `json:"assignment_id" gorm:"not null;index"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
