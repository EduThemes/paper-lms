package models

import "time"

type LTILineItem struct {
	ID                uint    `json:"id" gorm:"column:id;primaryKey"`
	AssignmentID      *uint   `json:"assignmentId" gorm:"index"` // Canvas assignment link (optional for standalone line items)
	ResourceLinkID    *uint   `json:"resource_link_id_internal" gorm:"index"`
	CourseID          uint    `json:"course_id" gorm:"not null;index"`
	Label             string  `json:"label" gorm:"not null"`
	ScoreMaximum      float64 `json:"scoreMaximum" gorm:"not null;default:100"`
	Tag               string  `json:"tag"`
	ResourceID        string  `json:"resourceId"`     // Tool-provided resource identifier
	ResourceLinkIDStr string  `json:"resourceLinkId"` // The LTI resource_link_id string
	LTISubmissionType string  `json:"https://canvas.instructure.com/lti/submission_type" gorm:"column:lti_submission_type"`
	// DeveloperKeyID is the tool (LTI developer key) that owns the line
	// item: one it created over AGS, or the platform-created default line
	// item of an external_tool assignment. Nil for line items created by
	// course staff through Paper's own API. A tool caller sees only its
	// own line items.
	DeveloperKeyID *uint      `json:"-" gorm:"column:developer_key_id;index"`
	StartDateTime  *time.Time `json:"startDateTime" gorm:"column:start_date_time"`
	EndDateTime    *time.Time `json:"endDateTime" gorm:"column:end_date_time"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
