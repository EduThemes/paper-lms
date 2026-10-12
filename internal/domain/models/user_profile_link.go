package models

import "time"

// UserProfileLink is one link a user shows on their profile (Canvas
// user_profile_links): a title and an http(s) URL.
type UserProfileLink struct {
	ID        uint      `json:"id" gorm:"column:id;primaryKey"`
	UserID    uint      `json:"user_id" gorm:"not null;index"`
	Title     string    `json:"title" gorm:"type:varchar(255);not null"`
	URL       string    `json:"url" gorm:"column:url;type:text;not null"`
	Position  int       `json:"position" gorm:"not null;default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
