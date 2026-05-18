package models

import "time"

type ContentTag struct {
	ID              uint               `json:"id" gorm:"column:id;primaryKey"`
	ContextModuleID uint               `json:"context_module_id" gorm:"not null;index"`
	ContentType     string             `json:"content_type" gorm:"not null"` // Assignment, WikiPage, ExternalUrl, ContextModuleSubHeader
	ContentID       *uint              `json:"content_id"`
	Title           string             `json:"title" gorm:"not null"`
	Position        int                `json:"position"`
	URL             string             `json:"url" gorm:"column:url"`
	Indent          int                `json:"indent" gorm:"default:0"`
	NewTab          bool               `json:"new_tab" gorm:"default:false"`
	WorkflowState   ContentTagWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'active'"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

func (ContentTag) TableName() string {
	return "content_tags"
}
