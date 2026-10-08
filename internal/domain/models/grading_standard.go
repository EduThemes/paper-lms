package models

import (
	"time"

	"gorm.io/gorm"
)

type GradingStandard struct {
	ID            uint   `json:"id" gorm:"column:id;primaryKey"`
	ContextType   string `json:"context_type" gorm:"not null"`
	ContextID     uint   `json:"context_id" gorm:"not null"`
	Title         string `json:"title" gorm:"not null"`
	Data          string `json:"data" gorm:"type:jsonb;not null"`
	WorkflowState string `json:"workflow_state" gorm:"not null;default:'active'"`
	// PointsBased schemes are shown in points out of ScalingFactor (e.g.
	// a 4.0 scale) instead of percentages; the stored cutoffs are still
	// fractions of 1.
	PointsBased   bool      `json:"points_based" gorm:"default:false"`
	ScalingFactor float64   `json:"scaling_factor" gorm:"default:1"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// BeforeCreate mirrors the SQL defaults GORM would otherwise overwrite
// with zero values ('' / 0).
func (g *GradingStandard) BeforeCreate(tx *gorm.DB) error {
	if g.WorkflowState == "" {
		g.WorkflowState = "active"
	}
	if g.ScalingFactor <= 0 {
		g.ScalingFactor = 1
	}
	return nil
}
