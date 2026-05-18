package models

import "time"

type LearningOutcome struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	ContextType       string    `json:"context_type" gorm:"not null"`
	ContextID         uint      `json:"context_id" gorm:"not null;index"`
	OutcomeGroupID    uint      `json:"outcome_group_id" gorm:"not null;index"`
	Title             string    `json:"title" gorm:"not null"`
	DisplayName       string    `json:"display_name"`
	Description       string    `json:"description" gorm:"type:text"`
	CalculationMethod string    `json:"calculation_method" gorm:"default:'decaying_average'"` // decaying_average, n_mastery, latest, highest
	CalculationInt    int       `json:"calculation_int" gorm:"default:65"`                    // param for calculation method
	MasteryPoints     float64   `json:"mastery_points" gorm:"default:3.0"`
	PointsPossible    float64   `json:"points_possible" gorm:"default:5.0"`
	RatingsData       string    `json:"ratings" gorm:"column:ratings_data;type:jsonb"` // JSON array of {points, description, color}
	WorkflowState     string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
