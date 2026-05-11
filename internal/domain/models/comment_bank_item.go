package models

import "time"

type CommentBankItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Comment   string    `gorm:"type:text;not null" json:"comment"`
	CourseID  *uint     `gorm:"index" json:"course_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
