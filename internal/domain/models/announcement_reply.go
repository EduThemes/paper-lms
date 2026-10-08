package models

import (
	"time"

	"gorm.io/gorm"
)

// Announcement reply workflow states.
const (
	AnnouncementReplyActive  = "active"
	AnnouncementReplyDeleted = "deleted"
)

// AnnouncementReply is a comment on an announcement that allows comments
// (Canvas announcements are discussion topics; their replies are entries).
// ParentID threads a reply under another reply (migration 000130).
type AnnouncementReply struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	AnnouncementID uint      `json:"announcement_id" gorm:"not null;index:idx_announcement_replies_announcement_id"`
	UserID         uint      `json:"user_id" gorm:"not null"`
	ParentID       *uint     `json:"parent_id" gorm:"index:idx_announcement_replies_parent_id"`
	Message        string    `json:"message" gorm:"type:text;not null"`
	WorkflowState  string    `json:"workflow_state" gorm:"type:text;not null;default:'active'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BeforeCreate mirrors the SQL DEFAULT.
func (r *AnnouncementReply) BeforeCreate(_ *gorm.DB) error {
	if r.WorkflowState == "" {
		r.WorkflowState = AnnouncementReplyActive
	}
	return nil
}

// AnnouncementSectionVisibility restricts an announcement whose
// target_audience is "sections" to the listed course sections (Canvas
// discussion_topic_section_visibilities; migration 000130).
type AnnouncementSectionVisibility struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	AnnouncementID  uint      `json:"announcement_id" gorm:"not null;uniqueIndex:idx_announcement_section,priority:1"`
	CourseSectionID uint      `json:"course_section_id" gorm:"not null;uniqueIndex:idx_announcement_section,priority:2;index:idx_announcement_section_visibilities_course_section_id"`
	CreatedAt       time.Time `json:"created_at"`
}
