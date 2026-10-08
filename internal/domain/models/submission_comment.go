package models

import "time"

type SubmissionComment struct {
	ID           uint      `json:"id" gorm:"column:id;primaryKey"`
	SubmissionID uint      `json:"submission_id" gorm:"not null"`
	AuthorID     uint      `json:"author_id" gorm:"not null"`
	Comment      string    `json:"comment" gorm:"type:text;not null"`
	Draft        bool      `json:"draft" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Attachments is a JSON array of the files (including recorded audio/
	// video) attached to the comment: [{id, display_name, content_type, size}].
	Attachments *string `json:"attachments" gorm:"type:text"`
	// GroupCommentID ties the copies of one comment posted to every member
	// of a group submission.
	GroupCommentID *string `json:"group_comment_id" gorm:"index"`
	// Attempt is the submission attempt the comment was left on.
	Attempt *int `json:"attempt"`
}
