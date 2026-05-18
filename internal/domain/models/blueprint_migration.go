package models

import "time"

// BlueprintMigration tracks a sync operation from a blueprint template to its associated courses.
type BlueprintMigration struct {
	ID                  uint       `json:"id" gorm:"column:id;primaryKey"`
	BlueprintTemplateID uint       `json:"blueprint_template_id" gorm:"not null;index"`
	UserID              uint       `json:"user_id" gorm:"not null"`
	WorkflowState       string     `json:"workflow_state" gorm:"not null;default:'queued'"` // queued, running, completed, failed
	Comment             string     `json:"comment" gorm:"type:text"`
	ExportSettings      string     `json:"export_settings" gorm:"type:jsonb;default:'{}'"`
	CompletedAt         *time.Time `json:"completed_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
