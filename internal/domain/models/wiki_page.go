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
	// date (Canvas wiki_pages.todo_date). Migration 000094.
	TodoDate *time.Time `json:"todo_date"`

	// CSS is the page stylesheet: sanitized (csssanitize.Sanitize) and
	// unscoped. It is scoped to #paper-page-<id> on output. Migration 000136.
	CSS string `json:"css" gorm:"column:css;type:text;not null;default:''"`
	// CSSSource is the stylesheet as the teacher typed it, with comments
	// and formatting, so the editor can show it back. Never served to
	// readers or applied: CSS is. It is shown only while it still
	// sanitizes to CSS (a copy or sync that rewrote CSS leaves it stale).
	CSSSource string `json:"-" gorm:"column:css_source;type:text;not null;default:''"`
	// Design is the Page Designer document (PageDoc) the page was made
	// from; NULL for an ordinary page. Migration 000136.
	Design NullJSON `json:"design" gorm:"column:design;type:jsonb"`
}

func (WikiPage) TableName() string {
	return "wiki_pages"
}
