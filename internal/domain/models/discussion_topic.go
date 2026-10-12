package models

import "time"

type DiscussionTopic struct {
	ID                 uint       `json:"id" gorm:"column:id;primaryKey"`
	CourseID           uint       `json:"course_id" gorm:"not null;index"`
	UserID             uint       `json:"user_id" gorm:"not null"`
	Title              string     `json:"title" gorm:"not null"`
	Message            string     `json:"message" gorm:"type:text"`
	DiscussionType     string     `json:"discussion_type" gorm:"default:'side_comment'"` // side_comment, threaded
	PostedAt           *time.Time `json:"posted_at"`
	DelayedPostAt      *time.Time `json:"delayed_post_at"`
	LockAt             *time.Time `json:"lock_at"`
	Pinned             bool       `json:"pinned" gorm:"default:false"`
	Locked             bool       `json:"locked" gorm:"default:false"`
	AllowRating        bool       `json:"allow_rating" gorm:"default:false"`
	OnlyGradersCanRate bool       `json:"only_graders_can_rate" gorm:"default:false"`
	SortByRating       bool       `json:"sort_by_rating" gorm:"default:false"`
	RequireInitialPost bool       `json:"require_initial_post" gorm:"default:false"`
	AssignmentID       *uint      `json:"assignment_id"`
	// GroupCategoryID makes this a group discussion: the topic is the
	// root, and every group in the set gets its own child topic
	// (RootTopicID = this id, GroupID = the group) where its members post.
	GroupCategoryID *uint `json:"group_category_id" gorm:"index"`
	RootTopicID     *uint `json:"root_topic_id" gorm:"index;uniqueIndex:idx_discussion_topics_root_group,priority:1,where:root_topic_id IS NOT NULL AND workflow_state <> 'deleted'"`
	GroupID         *uint `json:"group_id" gorm:"index;uniqueIndex:idx_discussion_topics_root_group,priority:2,where:root_topic_id IS NOT NULL AND workflow_state <> 'deleted'"`
	// AnonymousState is nil (names shown), "full_anonymity" (every
	// author is shown as an anonymous alias) or "partial_anonymity"
	// (each author chooses per post).
	AnonymousState *string `json:"anonymous_state" gorm:"type:text"`
	// IsSectionSpecific limits the topic to the sections listed in
	// discussion_topic_section_visibilities.
	IsSectionSpecific bool                    `json:"is_section_specific" gorm:"default:false"`
	WorkflowState     DiscussionTopicWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'active'"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}
