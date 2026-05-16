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

	// W3-B — per-enrollment leaderboard pseudonym.
	// PseudonymPoolCode chooses the word vocabulary
	// ("animals_v1" | "superheroes_v1" | "explorers_v1" | "first_name").
	// PseudonymName is the actual rendered alias, lazy-populated on first
	// leaderboard read. No `default:` GORM tag — migration 000043 carries
	// the SQL DEFAULT; explicit-on-INSERT keeps the bool-default class
	// of bug from reappearing for TEXT columns with policy semantics.
	//
	// PseudonymName is a *string (not string) so "not yet assigned"
	// serializes to SQL NULL — necessary because the partial UNIQUE
	// index on (course_id, pool_code, pseudonym_name) skips NULL rows,
	// and empty-string ("") would be indexed and collide on the first
	// two unassigned enrollments per course.
	PseudonymPoolCode string  `json:"pseudonym_pool_code" gorm:"not null"`
	PseudonymName     *string `json:"pseudonym_name,omitempty"`

	// Associations (not stored, loaded via joins)
	User   *User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Course *Course        `json:"course,omitempty" gorm:"foreignKey:CourseID"`
}
