package models

import "time"

type DocumentAnnotation struct {
	ID                 uint       `json:"id" gorm:"primaryKey"`
	SubmissionID       uint       `json:"submission_id" gorm:"not null;index;index:idx_annotation_submission_page,priority:1"`
	UserID             uint       `json:"user_id" gorm:"not null;index"`
	AnnotationType     string     `json:"annotation_type" gorm:"not null"` // highlight, comment, strikethrough, freehand, point
	Color              string     `json:"color" gorm:"default:'#FFFF00'"`  // hex color
	Content            string     `json:"content" gorm:"type:text"`        // comment text
	PageNumber         int        `json:"page_number" gorm:"default:1;index:idx_annotation_submission_page,priority:2"`
	SelectionStart     int        `json:"selection_start" gorm:"default:0"` // character position for text annotations
	SelectionEnd       int        `json:"selection_end" gorm:"default:0"`   // character position for text annotations
	X                  float64    `json:"x" gorm:"default:0"`               // for point/freehand/area annotations
	Y                  float64    `json:"y" gorm:"default:0"`
	Width              float64    `json:"width" gorm:"default:0"`
	Height             float64    `json:"height" gorm:"default:0"`
	PathData           string     `json:"path_data" gorm:"type:text"`  // SVG path data for freehand
	ParentAnnotationID *uint      `json:"parent_annotation_id" gorm:"index"` // for replies to annotations
	ResolvedAt         *time.Time `json:"resolved_at"`
	ResolvedByUserID   *uint      `json:"resolved_by_user_id"`
	WorkflowState      string     `json:"workflow_state" gorm:"not null;default:'active'"` // active, deleted, resolved
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Associations (not stored, loaded via joins)
	User             *User                `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Replies          []DocumentAnnotation `json:"replies,omitempty" gorm:"foreignKey:ParentAnnotationID"`
	ParentAnnotation *DocumentAnnotation  `json:"parent_annotation,omitempty" gorm:"foreignKey:ParentAnnotationID"`
}
