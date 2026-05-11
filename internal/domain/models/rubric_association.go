package models

import "time"

type RubricAssociation struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	RubricID           uint      `json:"rubric_id" gorm:"not null;index"`
	AssociationID      uint      `json:"association_id" gorm:"not null"`
	AssociationType    string    `json:"association_type" gorm:"not null"` // Assignment
	ContextType        string    `json:"context_type"`                     // Course
	ContextID          uint      `json:"context_id"`
	Purpose            string    `json:"purpose" gorm:"not null;default:'grading'"` // grading, bookmark
	UseForGrading      bool      `json:"use_for_grading" gorm:"default:false"`
	HideScoreTotal     bool      `json:"hide_score_total" gorm:"default:false"`
	HidePoints         bool      `json:"hide_points" gorm:"default:false"`
	HideOutcomeResults bool      `json:"hide_outcome_results" gorm:"default:false"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
