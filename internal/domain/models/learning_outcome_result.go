package models

import "time"

type LearningOutcomeResult struct {
	ID                  uint       `json:"id" gorm:"column:id;primaryKey"`
	UserID              uint       `json:"user_id" gorm:"not null;index"`
	LearningOutcomeID   uint       `json:"learning_outcome_id" gorm:"not null;index"`
	ContextType         string     `json:"context_type" gorm:"not null"`
	ContextID           uint       `json:"context_id" gorm:"not null"`
	AssociatedAssetType string     `json:"associated_asset_type"` // "Assignment", "Quiz"
	AssociatedAssetID   uint       `json:"associated_asset_id"`
	Score               *float64   `json:"score"`
	Possible            *float64   `json:"possible"`
	Mastery             *bool      `json:"mastery"`
	Percent             *float64   `json:"percent"`
	Attempt             int        `json:"attempt" gorm:"default:1"`
	AssessedAt          *time.Time `json:"assessed_at"`
	SubmittedAt         *time.Time `json:"submitted_at"`
	Title               string     `json:"title"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
