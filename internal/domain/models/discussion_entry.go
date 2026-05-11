package models

import "time"

type DiscussionEntry struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	DiscussionTopicID uint      `json:"discussion_topic_id" gorm:"not null;index"`
	UserID            uint      `json:"user_id" gorm:"not null;index"`
	ParentID          *uint     `json:"parent_id" gorm:"index"`
	Message           string    `json:"message" gorm:"type:text;not null"`
	RatingCount       int       `json:"rating_count" gorm:"default:0"`
	RatingSum         int       `json:"rating_sum" gorm:"default:0"`
	WorkflowState     string    `json:"workflow_state" gorm:"not null;default:'active';index"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
