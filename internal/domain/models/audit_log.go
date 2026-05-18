package models

import "time"

// AuditLog records all auditable actions in the system.
type AuditLog struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey"`
	EventType   string    `json:"event_type" gorm:"index"`   // grade_change, course_update, enrollment_change, assignment_update, submission_update, user_update, account_update
	UserID      uint      `json:"user_id" gorm:"index"`      // who performed the action
	CourseID    *uint     `json:"course_id" gorm:"index"`    // related course (if applicable)
	AccountID   *uint     `json:"account_id" gorm:"index"`   // related account
	ContextType string    `json:"context_type"`              // Submission, Course, Enrollment, Assignment, User, etc.
	ContextID   uint      `json:"context_id"`                // ID of the affected record
	Action      string    `json:"action"`                    // created, updated, deleted, graded, published, etc.
	Payload     string    `json:"payload" gorm:"type:jsonb"` // JSON with old/new values
	IPAddress   string    `json:"ip_address" gorm:"column:ip_address"`
	UserAgent   string    `json:"user_agent"`
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
}
