package models

import "time"

type LTILineItem struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	AssignmentID      *uint     `json:"assignmentId" gorm:"index"`                                                   // Canvas assignment link (optional for standalone line items)
	ResourceLinkID    *uint     `json:"resource_link_id_internal" gorm:"index"`
	CourseID          uint      `json:"course_id" gorm:"not null;index"`
	Label             string    `json:"label" gorm:"not null"`
	ScoreMaximum      float64   `json:"scoreMaximum" gorm:"not null;default:100"`
	Tag               string    `json:"tag"`
	ResourceID        string    `json:"resourceId"`                                                                   // Tool-provided resource identifier
	ResourceLinkIDStr string    `json:"resourceLinkId"`                                                               // The LTI resource_link_id string
	LTISubmissionType string    `json:"https://canvas.instructure.com/lti/submission_type"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
