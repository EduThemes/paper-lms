package models

import "time"

// StudentAccommodation represents a student's IEP/504 accommodation profile
// Unlike Canvas which requires manual per-assignment overrides, this auto-applies
type StudentAccommodation struct {
	ID                uint   `json:"id" gorm:"column:id;primaryKey"`
	UserID            uint   `json:"user_id" gorm:"not null;index"`
	CourseID          *uint  `json:"course_id" gorm:"index"`             // nil = applies to all courses
	AccommodationType string `json:"accommodation_type" gorm:"not null"` // extended_time, modified_due_dates, alternative_format, reduced_assignments, preferential_seating, assistive_tech, other
	Description       string `json:"description" gorm:"type:text"`
	// Extended time settings
	TimeMultiplier *float64 `json:"time_multiplier"` // e.g., 1.5 for time-and-a-half on quizzes
	ExtraDays      *int     `json:"extra_days"`      // extra days for assignment due dates
	// Status
	Status         string `json:"status" gorm:"not null;default:'active'"` // active, inactive, expired
	PlanType       string `json:"plan_type"`                               // IEP, 504, ELL, gifted, informal, other
	PlanExternalID string `json:"plan_external_id"`                        // reference to external IEP/504 system
	// Approval chain
	CreatedByID    uint       `json:"created_by_id" gorm:"not null"`
	ApprovedByID   *uint      `json:"approved_by_id"`
	ApprovedAt     *time.Time `json:"approved_at"`
	EffectiveFrom  time.Time  `json:"effective_from" gorm:"not null"`
	EffectiveUntil *time.Time `json:"effective_until"`
	// Audit
	Notes     string    `json:"notes" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AccommodationApplication tracks when an accommodation was auto-applied
type AccommodationApplication struct {
	ID                uint       `json:"id" gorm:"column:id;primaryKey"`
	AccommodationID   uint       `json:"accommodation_id" gorm:"not null;index"`
	ResourceType      string     `json:"resource_type" gorm:"not null"` // assignment, quiz
	ResourceID        uint       `json:"resource_id" gorm:"not null;index"`
	UserID            uint       `json:"user_id" gorm:"not null;index"`
	OriginalDueAt     *time.Time `json:"original_due_at"`
	AdjustedDueAt     *time.Time `json:"adjusted_due_at"`
	OriginalTimeLimit *int       `json:"original_time_limit"` // minutes
	AdjustedTimeLimit *int       `json:"adjusted_time_limit"` // minutes
	AppliedAt         time.Time  `json:"applied_at"`
	CreatedAt         time.Time  `json:"created_at"`
}
