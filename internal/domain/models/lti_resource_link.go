package models

import "time"

type LTIResourceLink struct {
	ID                    uint      `json:"id" gorm:"primaryKey"`
	ContextExternalToolID uint      `json:"context_external_tool_id" gorm:"not null;index"`
	ContextType           string    `json:"context_type" gorm:"not null"`
	ContextID             uint      `json:"context_id" gorm:"not null"`
	ResourceLinkID        string    `json:"resource_link_id" gorm:"uniqueIndex;not null"` // UUID
	Title                 string    `json:"title"`
	URL                   string    `json:"url"`
	CustomParameters      string    `json:"custom" gorm:"type:text"`
	LookupUUID            string    `json:"lookup_uuid" gorm:"uniqueIndex"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
