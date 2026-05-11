package models

import "time"

type Nonce struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Value     string    `json:"value" gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null;index"`
	CreatedAt time.Time `json:"created_at"`
}
