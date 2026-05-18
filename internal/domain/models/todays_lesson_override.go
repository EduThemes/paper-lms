package models

import "time"

type TodaysLessonOverride struct {
	ID       uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID uint      `json:"course_id" gorm:"not null;uniqueIndex:idx_course_date"`
	Date     time.Time `json:"date" gorm:"type:date;not null;uniqueIndex:idx_course_date"`
	LinkType string    `json:"link_type" gorm:"not null"`
	LinkID   *uint     `json:"link_id"`
	LinkURL  string    `json:"link_url"`
	Label    string    `json:"label"`
}
