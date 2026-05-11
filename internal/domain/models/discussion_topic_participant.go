package models

import "time"

type DiscussionTopicParticipant struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	DiscussionTopicID uint       `json:"discussion_topic_id" gorm:"not null;uniqueIndex:idx_topic_user"`
	UserID            uint       `json:"user_id" gorm:"not null;uniqueIndex:idx_topic_user"`
	Subscribed        bool       `json:"subscribed" gorm:"default:true"`
	ForcedReadState   *string    `json:"forced_read_state"` // "read" or nil
	LastReadAt        *time.Time `json:"last_read_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
