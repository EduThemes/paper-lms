package models

import "time"

type NotificationPreference struct {
	ID                    uint      `json:"id" gorm:"column:id;primaryKey"`
	UserID                uint      `json:"user_id" gorm:"not null;uniqueIndex"`
	Policy                string    `json:"policy" gorm:"default:'daily'"`
	NotifyNewMessage      bool      `json:"notify_new_message" gorm:"default:true"`
	NotifyEventStart      bool      `json:"notify_event_start" gorm:"default:false"`
	NotifySubmissionGrade bool      `json:"notify_submission_grade" gorm:"default:true"`
	NotifyNewAnnouncement bool      `json:"notify_new_announcement" gorm:"default:true"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
