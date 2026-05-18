package models

import "time"

type DiscussionEntryParticipant struct {
	ID                uint       `json:"id" gorm:"column:id;primaryKey"`
	DiscussionEntryID uint       `json:"discussion_entry_id" gorm:"not null;uniqueIndex:idx_entry_user"`
	UserID            uint       `json:"user_id" gorm:"not null;uniqueIndex:idx_entry_user"`
	ReadAt            *time.Time `json:"read_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
