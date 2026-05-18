package models

import "time"

type Portfolio struct {
	ID             uint       `json:"id" gorm:"column:id;primaryKey"`
	UserID         uint       `json:"user_id" gorm:"not null;index"`
	Title          string     `json:"title" gorm:"not null"`
	Slug           string     `json:"slug" gorm:"uniqueIndex;not null"` // URL-friendly identifier
	Description    string     `json:"description" gorm:"type:text"`
	ThemeID        string     `json:"theme_id" gorm:"not null;default:'clean-modern'"` // clean-modern, creative-bold, academic-classic, minimal-dark, portfolio-developer
	CustomCSS      string     `json:"custom_css" gorm:"type:text"`
	HeaderImageURL string     `json:"header_image_url"`
	AvatarURL      string     `json:"avatar_url"`
	Tagline        string     `json:"tagline"`
	ContactEmail   string     `json:"contact_email"`
	LinkedInURL    string     `json:"linkedin_url"`
	WebsiteURL     string     `json:"website_url"`
	IsPublic       bool       `json:"is_public" gorm:"default:false"`
	PublicURL      string     `json:"public_url" gorm:"uniqueIndex"`
	CustomDomain   string     `json:"custom_domain"`
	WorkflowState  string     `json:"workflow_state" gorm:"not null;default:'draft'"` // draft, published, archived
	ViewCount      int64      `json:"view_count" gorm:"default:0"`
	LastExportedAt *time.Time `json:"last_exported_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type PortfolioSection struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey"`
	PortfolioID uint      `json:"portfolio_id" gorm:"not null;index"`
	Title       string    `json:"title" gorm:"not null"`
	SectionType string    `json:"section_type" gorm:"not null"` // about, projects, experience, education, skills, gallery, blog, custom
	Content     string    `json:"content" gorm:"type:text"`     // Rich HTML content
	Position    int       `json:"position" gorm:"not null;default:0"`
	IsVisible   bool      `json:"is_visible" gorm:"default:true"`
	Layout      string    `json:"layout" gorm:"default:'standard'"` // standard, two-column, grid, timeline, masonry
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PortfolioArtifact struct {
	ID           uint   `json:"id" gorm:"column:id;primaryKey"`
	PortfolioID  uint   `json:"portfolio_id" gorm:"not null;index"`
	SectionID    *uint  `json:"section_id" gorm:"index"`
	Title        string `json:"title" gorm:"not null"`
	Description  string `json:"description" gorm:"type:text"`
	ArtifactType string `json:"artifact_type" gorm:"not null"` // project, document, image, video, link, course_work, reflection, certificate
	ContentURL   string `json:"content_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	// Source tracking - where this artifact came from
	SourceType     string `json:"source_type"` // upload, course_submission, course_page, external_link
	SourceCourseID *uint  `json:"source_course_id"`
	SourceID       *uint  `json:"source_id"` // assignment_id, submission_id, etc.
	// Metadata
	FileType      string `json:"file_type"` // pdf, png, jpg, mp4, etc.
	FileSizeBytes int64  `json:"file_size_bytes"`
	Tags          string `json:"tags" gorm:"type:text"` // JSON array of tags
	// Outcome alignment
	OutcomeIDs string    `json:"outcome_ids" gorm:"type:text"` // JSON array of learning outcome IDs
	Position   int       `json:"position" gorm:"default:0"`
	IsFeatured bool      `json:"is_featured" gorm:"default:false"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PortfolioReflection struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey"`
	ArtifactID uint      `json:"artifact_id" gorm:"not null;index"`
	UserID     uint      `json:"user_id" gorm:"not null"`
	PromptText string    `json:"prompt_text" gorm:"type:text"`      // teacher-provided reflection prompt
	Content    string    `json:"content" gorm:"type:text;not null"` // student's reflection
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PortfolioTemplate struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID   *uint     `json:"account_id" gorm:"index"` // nil = system template
	CreatedByID uint      `json:"created_by_id"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description" gorm:"type:text"`
	ThemeID     string    `json:"theme_id" gorm:"not null"`
	Sections    string    `json:"sections" gorm:"type:text"` // JSON definition of sections to create
	IsPublic    bool      `json:"is_public" gorm:"default:false"`
	UsageCount  int       `json:"usage_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PortfolioComment - for teacher/peer feedback
type PortfolioComment struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey"`
	PortfolioID uint      `json:"portfolio_id" gorm:"not null;index"`
	SectionID   *uint     `json:"section_id" gorm:"index"`
	ArtifactID  *uint     `json:"artifact_id" gorm:"index"`
	UserID      uint      `json:"user_id" gorm:"not null"`
	Content     string    `json:"content" gorm:"type:text;not null"`
	ParentID    *uint     `json:"parent_id"` // for threaded replies
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
