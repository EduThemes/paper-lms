package models

import "time"

type DiscussionEntryVersion struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	DiscussionEntryID uint      `json:"discussion_entry_id" gorm:"not null;index"`
	UserID            uint      `json:"user_id" gorm:"not null"`
	Message           string    `json:"message" gorm:"type:text;not null"`
	Version           int       `json:"version" gorm:"not null"`
	CreatedAt         time.Time `json:"created_at"`
}
