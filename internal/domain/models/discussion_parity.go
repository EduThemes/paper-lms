package models

import "time"

// Anonymous discussion states (DiscussionTopic.AnonymousState).
const (
	DiscussionFullAnonymity    = "full_anonymity"
	DiscussionPartialAnonymity = "partial_anonymity"
)

// DiscussionTopicSectionVisibility ties a section-specific discussion
// topic to one of the course sections that may see it.
type DiscussionTopicSectionVisibility struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	DiscussionTopicID uint      `json:"discussion_topic_id" gorm:"not null;uniqueIndex:idx_dtsv_topic_section"`
	CourseSectionID   uint      `json:"course_section_id" gorm:"not null;uniqueIndex:idx_dtsv_topic_section;index"`
	CreatedAt         time.Time `json:"created_at"`
}

// DiscussionEntryMention records that a post @mentioned a user, so an
// edit only notifies people who were newly mentioned.
type DiscussionEntryMention struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	DiscussionEntryID uint      `json:"discussion_entry_id" gorm:"not null;uniqueIndex:idx_dem_entry_user"`
	UserID            uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_dem_entry_user;index"`
	CreatedAt         time.Time `json:"created_at"`
}
