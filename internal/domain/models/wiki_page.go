package models

import "time"

type WikiPage struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID      uint      `json:"course_id" gorm:"not null;index"`
	Title         string    `json:"title" gorm:"not null"`
	URL           string    `json:"url" gorm:"column:url;not null;index"` // slug
	Body          string    `json:"body" gorm:"type:text"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'unpublished';index"`
	EditingRoles  string    `json:"editing_roles" gorm:"default:'teachers'"`
	FrontPage     bool      `json:"front_page" gorm:"default:false"`
	Public        bool      `json:"public" gorm:"default:false"`
	WebsiteMode   bool      `json:"website_mode" gorm:"default:false"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// TodoDate puts an ungraded page on learners' to-do lists for that
	// date (Canvas wiki_pages.todo_date). Migration 000113.
	TodoDate *time.Time `json:"todo_date"`
}

func (WikiPage) TableName() string {
	return "wiki_pages"
}
