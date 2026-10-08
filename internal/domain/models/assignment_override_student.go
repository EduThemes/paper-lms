package models

import "time"

type AssignmentOverrideStudent struct {
	ID                   uint      `json:"id" gorm:"column:id;primaryKey"`
	AssignmentOverrideID uint      `json:"assignment_override_id" gorm:"not null;index"`
	// A student is in at most one override per assignment (Canvas
	// parity; migration 000081 replaced the instance-wide
	// UNIQUE(user_id) idx_override_user).
	UserID       uint `json:"user_id" gorm:"not null;uniqueIndex:idx_override_assignment_user,priority:2"`
	AssignmentID uint `json:"assignment_id" gorm:"not null;index;uniqueIndex:idx_override_assignment_user,priority:1"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
