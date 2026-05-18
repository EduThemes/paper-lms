package models

import "time"

type RubricAssessment struct {
	ID                  uint      `json:"id" gorm:"column:id;primaryKey"`
	RubricID            uint      `json:"rubric_id" gorm:"not null;index"`
	RubricAssociationID uint      `json:"rubric_association_id" gorm:"not null"`
	UserID              uint      `json:"user_id" gorm:"not null"`     // student being assessed
	AssessorID          uint      `json:"assessor_id" gorm:"not null"` // teacher/assessor
	Score               *float64  `json:"score"`
	Data                string    `json:"data" gorm:"type:jsonb"`                            // {criterion_id: {points, comments}}
	AssessmentType      string    `json:"assessment_type" gorm:"not null;default:'grading'"` // grading, peer_review
	WorkflowState       string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
