package models

import "time"

// StudentViewStudent links a course to its "Test Student" (Canvas Student
// View / StudentViewEnrollment). The test student is an ordinary user in
// the course's account with an active StudentEnrollment; this row is what
// marks it fake. Staff enter Student View by switching their session to
// that user (a masquerade capped at the test student's own rights). One
// test student per course; "reset" deletes the row and retires the user's
// enrollment so the next entry creates a fresh one. Migration 000096.
type StudentViewStudent struct {
	ID        uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID  uint      `json:"course_id" gorm:"not null;uniqueIndex"`
	UserID    uint      `json:"user_id" gorm:"not null;uniqueIndex"`
	CreatedAt time.Time `json:"created_at"`
}

func (StudentViewStudent) TableName() string {
	return "student_view_students"
}
