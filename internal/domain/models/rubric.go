package models

import "time"

type Rubric struct {
	ID                        uint      `json:"id" gorm:"column:id;primaryKey"`
	ContextType               string    `json:"context_type" gorm:"not null"` // Course, Account
	ContextID                 uint      `json:"context_id" gorm:"not null"`
	Title                     string    `json:"title" gorm:"not null"`
	Description               string    `json:"description" gorm:"type:text"`
	Data                      string    `json:"data" gorm:"type:jsonb"` // [{id, description, long_description, points, ratings: [{id, points, description}]}]
	PointsPossible            float64   `json:"points_possible"`
	FreeFormCriterionComments bool      `json:"free_form_criterion_comments" gorm:"default:false"`
	HideScoreTotal            bool      `json:"hide_score_total" gorm:"default:false"`
	HidePoints                bool      `json:"hide_points" gorm:"default:false"`
	WorkflowState             string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}
