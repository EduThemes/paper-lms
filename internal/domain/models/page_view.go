package models

import "time"

type PageView struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	UserID             uint      `json:"user_id" gorm:"not null;index"`
	ContextType        string    `json:"context_type" gorm:"not null"`
	ContextID          uint      `json:"context_id" gorm:"not null;index"`
	URL                string    `json:"url"`
	Action             string    `json:"action"`
	Participated       bool      `json:"participated" gorm:"default:false"`
	InteractionSeconds int       `json:"interaction_seconds" gorm:"default:0"`
	CreatedAt          time.Time `json:"created_at"`
}
