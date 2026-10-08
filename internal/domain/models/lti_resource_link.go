package models

import "time"

// LTIResourceLink is one LTI 1.3 resource link: the stable identity a tool
// sees as the resource_link claim's id (ResourceLinkID, a UUID) for a
// placement of a tool in a course.
//
// ContextType/ContextID are the tenant anchor ("Course" + course id). A
// link is additionally attached to the Paper object it belongs to: an
// assignment (external_tool submission type) or a module item
// (ContextExternalTool content tag). Course-level links created by the
// rich content editor or link_selection carry neither.
type LTIResourceLink struct {
	ID                    uint      `json:"id" gorm:"column:id;primaryKey"`
	ContextExternalToolID uint      `json:"context_external_tool_id" gorm:"not null;index"`
	ContextType           string    `json:"context_type" gorm:"not null"`
	ContextID             uint      `json:"context_id" gorm:"not null"`
	ResourceLinkID        string    `json:"resource_link_id" gorm:"uniqueIndex;not null"` // UUID
	Title                 string    `json:"title"`
	URL                   string    `json:"url" gorm:"column:url"`
	CustomParameters      string    `json:"custom" gorm:"type:text"`
	LookupUUID            string    `json:"lookup_uuid" gorm:"uniqueIndex"`
	AssignmentID          *uint     `json:"assignment_id" gorm:"column:assignment_id;index"`
	ContentTagID          *uint     `json:"content_tag_id" gorm:"column:content_tag_id;index"`
	Placement             string    `json:"placement" gorm:"column:placement"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
