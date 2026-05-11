package models

import "time"

type SubmissionComment struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SubmissionID uint      `json:"submission_id" gorm:"not null"`
	AuthorID     uint      `json:"author_id" gorm:"not null"`
	Comment      string    `json:"comment" gorm:"type:text;not null"`
	Draft        bool      `json:"draft" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
