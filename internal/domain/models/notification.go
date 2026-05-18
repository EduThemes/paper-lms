package models

import "time"

type Notification struct {
	ID               uint       `json:"id" gorm:"column:id;primaryKey"`
	UserID           uint       `json:"user_id" gorm:"not null;index"`
	NotificationType string     `json:"notification_type" gorm:"not null"`
	Title            string     `json:"title" gorm:"not null"`
	Message          string     `json:"message" gorm:"type:text"`
	ContextType      string     `json:"context_type"`
	ContextID        uint       `json:"context_id"`
	RelatedUserID    *uint      `json:"related_user_id"`
	IsRead           bool       `json:"is_read" gorm:"default:false;index"`
	SentAt           *time.Time `json:"sent_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
