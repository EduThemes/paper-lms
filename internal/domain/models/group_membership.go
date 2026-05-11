package models

import "time"

type GroupMembership struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	GroupID       uint      `json:"group_id" gorm:"not null;uniqueIndex:idx_group_user"`
	UserID        uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_group_user"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'accepted'"` // accepted, invited, requested
	Moderator     bool      `json:"moderator" gorm:"default:false"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}
