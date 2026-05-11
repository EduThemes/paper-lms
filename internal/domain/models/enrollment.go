package models

import "time"

type Enrollment struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	UserID          uint       `json:"user_id" gorm:"not null;index"`
	CourseID        uint       `json:"course_id" gorm:"not null;index"`
	CourseSectionID *uint      `json:"course_section_id" gorm:"index"`
	Type            string     `json:"type" gorm:"not null"` // StudentEnrollment, TeacherEnrollment, TaEnrollment, ObserverEnrollment, DesignerEnrollment
	Role            string     `json:"role" gorm:"not null"`
	WorkflowState   string     `json:"workflow_state" gorm:"not null;default:'active';index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastActivityAt   *time.Time `json:"last_activity_at"`
	AssociatedUserID *uint      `json:"associated_user_id" gorm:"index"` // For ObserverEnrollment linking

	// Associations (not stored, loaded via joins)
	User   *User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Course *Course        `json:"course,omitempty" gorm:"foreignKey:CourseID"`
}
