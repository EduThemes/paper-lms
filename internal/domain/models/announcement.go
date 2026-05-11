package models

import "time"

type Announcement struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	CourseID       *uint      `json:"course_id" gorm:"index"`
	AccountID      *uint      `json:"account_id" gorm:"index"`
	UserID         uint       `json:"user_id"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	Priority       string     `json:"priority" gorm:"default:normal"`
	RequireAck     bool       `json:"require_acknowledgement" gorm:"default:false"`
	TargetAudience string     `json:"target_audience" gorm:"default:all"`
	PostedAt       *time.Time `json:"posted_at"`
	DelayedPostAt  *time.Time `json:"delayed_post_at"`
	WorkflowState  string     `json:"workflow_state" gorm:"default:draft"`
	AllowComments  bool       `json:"allow_comments" gorm:"default:false"`
	IsGlobal       bool       `json:"is_global" gorm:"default:false"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
