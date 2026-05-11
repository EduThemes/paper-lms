package models

import "time"

type DiscussionTopic struct {
	ID                 uint       `json:"id" gorm:"primaryKey"`
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
	WorkflowState      string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
