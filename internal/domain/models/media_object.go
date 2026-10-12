package models

import (
	"time"

	"gorm.io/gorm"
)

// MediaObjectWorkflow is the row lifecycle of a MediaObject.
type MediaObjectWorkflow string

const (
	MediaObjectActive  MediaObjectWorkflow = "active"
	MediaObjectDeleted MediaObjectWorkflow = "deleted"
)

// MediaObject is an uploaded or browser-recorded audio/video clip that rich
// content embeds (Canvas media_objects, G002). The bytes live in an
// Attachment owned by the uploader (ContextType "User"); the media object
// adds the player-facing identity: an unguessable MediaID that rich content
// references (/api/v1/media_objects/:media_id/stream) and the caption tracks.
//
// Access model (mirrors Canvas, whose media ids are effectively public
// identifiers): any signed-in user of the same tenant who has the media id
// can play it — they saw it in content they were allowed to read. Editing
// the title and captions is limited to the owner, tenant admins and, for
// media uploaded in a course, that course's instructors; deleting to the
// owner and tenant admins.
type MediaObject struct {
	ID           uint   `json:"id" gorm:"column:id;primaryKey"`
	AccountID    uint   `json:"-" gorm:"not null;index"`
	UserID       uint   `json:"user_id" gorm:"not null;index"`
	AttachmentID uint   `json:"attachment_id" gorm:"not null;index"`
	MediaID      string `json:"media_id" gorm:"type:text;not null;uniqueIndex:idx_media_objects_media_id"`
	// ContextType is "Course" (uploaded from a course's content) or "User".
	ContextType   string              `json:"context_type" gorm:"type:text;not null"`
	ContextID     uint                `json:"context_id" gorm:"not null"`
	MediaType     string              `json:"media_type" gorm:"type:text;not null"` // "video" | "audio"
	ContentType   string              `json:"content_type" gorm:"type:text;not null"`
	Title         string              `json:"title" gorm:"type:text;not null;default:''"`
	WorkflowState MediaObjectWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'active'"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

func (MediaObject) TableName() string { return "media_objects" }

// BeforeCreate mirrors the SQL default (GORM would write an empty string).
func (m *MediaObject) BeforeCreate(_ *gorm.DB) error {
	if m.WorkflowState == "" {
		m.WorkflowState = MediaObjectActive
	}
	return nil
}

// MediaTrack is one caption / subtitle track of a MediaObject (Canvas
// media_tracks). Content keeps what was uploaded (SRT or WebVTT);
// WebVTTContent is what players load. One track per locale per media object.
type MediaTrack struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	MediaObjectID uint      `json:"media_object_id" gorm:"not null;uniqueIndex:idx_media_tracks_object_locale,priority:1"`
	AccountID     uint      `json:"-" gorm:"not null;index"`
	UserID        uint      `json:"user_id" gorm:"not null"`
	Kind          string    `json:"kind" gorm:"type:text;not null;default:'subtitles'"`
	Locale        string    `json:"locale" gorm:"type:text;not null;uniqueIndex:idx_media_tracks_object_locale,priority:2"`
	Label         string    `json:"label" gorm:"type:text;not null;default:''"`
	Content       string    `json:"-" gorm:"type:text;not null"`
	WebVTTContent string    `json:"-" gorm:"column:webvtt_content;type:text;not null"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (MediaTrack) TableName() string { return "media_tracks" }

// BeforeCreate mirrors the SQL default for kind.
func (t *MediaTrack) BeforeCreate(_ *gorm.DB) error {
	if t.Kind == "" {
		t.Kind = "subtitles"
	}
	return nil
}
