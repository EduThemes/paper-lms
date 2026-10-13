package models

import "time"

// Calendar event context types. A CourseSection event is always a child
// of a Course event (ParentCalendarEventID set): the parent carries the
// shared title/description and each child carries one section's times.
const (
	CalendarContextUser          = "User"
	CalendarContextCourse        = "Course"
	CalendarContextCourseSection = "CourseSection"
	CalendarContextGroup         = "Group"
	CalendarContextAccount       = "Account"
)

type CalendarEvent struct {
	ID              uint       `json:"id" gorm:"column:id;primaryKey"`
	ContextType     string     `json:"context_type" gorm:"not null;index:idx_cal_event_context"`
	ContextID       uint       `json:"context_id" gorm:"not null;index:idx_cal_event_context"`
	Title           string     `json:"title" gorm:"not null"`
	Description     string     `json:"description" gorm:"type:text"`
	StartAt         time.Time  `json:"start_at" gorm:"not null;index"`
	EndAt           *time.Time `json:"end_at"`
	LocationName    string     `json:"location_name"`
	LocationAddress string     `json:"location_address"`
	AllDay          bool       `json:"all_day" gorm:"default:false"`
	CreatedByUserID uint       `json:"created_by_user_id" gorm:"not null"`
	WorkflowState   string     `json:"workflow_state" gorm:"default:'active'"`

	// ParentCalendarEventID ties a section-specific child event to its
	// course-level parent. A parent with live children is a container:
	// the calendar shows the children, never the parent itself.
	ParentCalendarEventID *uint `json:"parent_calendar_event_id" gorm:"index:idx_cal_event_parent"`

	// Recurring series. Every occurrence is its own row sharing
	// SeriesUUID; the first one is the SeriesHead and every row carries
	// the RFC 5545 RRULE the series was generated from.
	SeriesUUID *string `json:"series_uuid" gorm:"column:series_uuid;type:text;index:idx_cal_event_series"`
	RRule      string  `json:"rrule" gorm:"column:rrule;type:text"`
	SeriesHead bool    `json:"series_head" gorm:"default:false"`

	// TimeZone is the IANA zone the event was created in (migration
	// 000134). An all-day event's date is read in it; NULL (older rows)
	// falls back to the viewer's profile zone.
	TimeZone *string `json:"time_zone" gorm:"column:time_zone;type:text"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CalendarFeedToken is the per-user secret behind the subscribable ICS
// feed URL (/feeds/calendars/<token>.ics). Only a SHA-256 digest is used
// for lookup; the token itself is kept sealed with secretbox so the
// calendar page can show the URL again. Rotating the token revokes every
// subscription made with the old URL.
type CalendarFeedToken struct {
	ID              uint       `json:"id" gorm:"column:id;primaryKey"`
	UserID          uint       `json:"user_id" gorm:"not null;uniqueIndex:idx_cal_feed_token_user"`
	TokenDigest     string     `json:"-" gorm:"type:text;not null;uniqueIndex:idx_cal_feed_token_digest"`
	TokenCiphertext []byte     `json:"-" gorm:"column:token_ciphertext"`
	LastAccessedAt  *time.Time `json:"last_accessed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (CalendarFeedToken) TableName() string { return "calendar_feed_tokens" }
