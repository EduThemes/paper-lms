package models

import "time"

// SharedContent represents a piece of content published to the Commons
// (Canvas Commons equivalent). District-scoped via AccountID. The actual
// payload is stored as JSON in ContentSnapshot — either a single resource
// or, for resource_type="course", a multi-resource bundle. The schema is
// intentionally compatible with the existing IMSCC content_migrations
// pipeline so an importer can reuse it (Commons import = a content_migration
// with source=commons).
type SharedContent struct {
	ID              uint   `json:"id" gorm:"column:id;primaryKey"`
	AccountID       uint   `json:"account_id" gorm:"not null;default:1;index"`
	AuthorUserID    uint   `json:"author_user_id" gorm:"not null;index"`
	Title           string `json:"title" gorm:"not null"`
	Description     string `json:"description" gorm:"type:text"`
	ResourceType    string `json:"resource_type" gorm:"not null;index"` // course | assignment | page | quiz | module | discussion_topic
	SourceCourseID  uint   `json:"source_course_id" gorm:"index"`
	SourceContentID *uint  `json:"source_content_id" gorm:"index"` // null for full-course exports
	Subject         string `json:"subject" gorm:"index"`           // e.g. "Math", "ELA"
	GradeLevel      string `json:"grade_level" gorm:"index"`       // K-2, 3-5, 6-8, 9-12
	// Tags is stored as a JSON array of strings (jsonb) so we don't depend
	// on the pq Postgres array driver here.
	Tags            string    `json:"tags" gorm:"type:jsonb;default:'[]'"`
	ThumbnailURL    string    `json:"thumbnail_url"`
	ContentSnapshot string    `json:"content_snapshot,omitempty" gorm:"type:jsonb"` // serialized payload
	DownloadCount   int       `json:"download_count" gorm:"default:0"`
	FavoriteCount   int       `json:"favorite_count" gorm:"default:0"`
	Visibility      string    `json:"visibility" gorm:"default:'account'"` // account | public
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (SharedContent) TableName() string {
	return "shared_content"
}

// SharedContentFavorite is a join table — a teacher favoriting a Commons
// resource. Unique on (shared_content_id, user_id).
type SharedContentFavorite struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	SharedContentID uint      `json:"shared_content_id" gorm:"not null;uniqueIndex:idx_shared_fav_unique,priority:1"`
	UserID          uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_shared_fav_unique,priority:2;index"`
	CreatedAt       time.Time `json:"created_at"`
}

func (SharedContentFavorite) TableName() string {
	return "shared_content_favorites"
}
