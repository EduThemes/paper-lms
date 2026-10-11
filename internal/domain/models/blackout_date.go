package models

import "time"

// BlackoutDate is a span of days on which course pacing schedules no
// work (Canvas BlackoutDate: holidays, breaks). Course pacing skips
// every day from StartDate through EndDate, inclusive, the same way it
// skips weekends when a pace excludes them. Migration 000093.
type BlackoutDate struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID   uint      `json:"course_id" gorm:"not null;index:idx_blackout_dates_course_id"`
	EventTitle string    `json:"event_title" gorm:"type:text;not null"`
	StartDate  time.Time `json:"start_date" gorm:"type:date;not null"`
	EndDate    time.Time `json:"end_date" gorm:"type:date;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (BlackoutDate) TableName() string { return "blackout_dates" }

// Covers reports whether day (compared by calendar date in day's
// location) falls inside the blackout span.
func (b BlackoutDate) Covers(day time.Time) bool {
	d := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	start := time.Date(b.StartDate.Year(), b.StartDate.Month(), b.StartDate.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(b.EndDate.Year(), b.EndDate.Month(), b.EndDate.Day(), 0, 0, 0, 0, time.UTC)
	return !d.Before(start) && !d.After(end)
}
