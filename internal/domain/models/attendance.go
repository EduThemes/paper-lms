package models

import "time"

type AttendanceRecord struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID   uint      `json:"course_id" gorm:"not null;index"`
	SectionID  *uint     `json:"section_id" gorm:"index"`
	UserID     uint      `json:"user_id" gorm:"not null;index"`
	Date       time.Time `json:"date" gorm:"not null;index;type:date"`
	Status     string    `json:"status" gorm:"not null"` // present, absent, tardy, excused
	Notes      string    `json:"notes" gorm:"type:text"`
	MarkedByID uint      `json:"marked_by_id" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AttendanceSummary struct {
	UserID         uint    `json:"user_id"`
	CourseID       uint    `json:"course_id"`
	TotalDays      int     `json:"total_days"`
	PresentDays    int     `json:"present_days"`
	AbsentDays     int     `json:"absent_days"`
	TardyDays      int     `json:"tardy_days"`
	ExcusedDays    int     `json:"excused_days"`
	AttendanceRate float64 `json:"attendance_rate"`
}
