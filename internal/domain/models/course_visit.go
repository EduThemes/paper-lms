package models

import "time"

type CourseVisit struct {
	ID        uint      `json:"id" gorm:"column:id;primaryKey"`
	UserID    uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_user_course"`
	CourseID  uint      `json:"course_id" gorm:"not null;uniqueIndex:idx_user_course"`
	LastURL   string    `json:"last_url"`
	LastTitle string    `json:"last_title"`
	UpdatedAt time.Time `json:"updated_at"`
}
