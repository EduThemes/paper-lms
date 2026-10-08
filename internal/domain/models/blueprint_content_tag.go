package models

import "time"

// BlueprintContentTag links a piece of blueprint (source) course content
// to the copy a sync created in one associated (child) course — Paper's
// equivalent of Canvas's MasterContentTag/ChildContentTag pair.
//
// Sync uses it to find the child copy of a source item by identity
// rather than by title (so renames propagate instead of duplicating),
// and SyncedAt records the child row's state as of the last sync so a
// later local edit in the child course ("downstream change") can be
// detected and, unless the blueprint locks content, preserved.
//
// ContentType uses the canonical names of service.contentKey:
// "Assignment", "WikiPage", "Quiz", "DiscussionTopic", "ContextModule",
// "ContentTag".
type BlueprintContentTag struct {
	ID                  uint      `json:"id" gorm:"column:id;primaryKey"`
	BlueprintTemplateID uint      `json:"blueprint_template_id" gorm:"not null;uniqueIndex:idx_bp_content_tag_source,priority:1"`
	ChildCourseID       uint      `json:"child_course_id" gorm:"not null;uniqueIndex:idx_bp_content_tag_source,priority:2"`
	ContentType         string    `json:"content_type" gorm:"type:text;not null;uniqueIndex:idx_bp_content_tag_source,priority:3"`
	SourceID            uint      `json:"source_id" gorm:"not null;uniqueIndex:idx_bp_content_tag_source,priority:4"`
	ChildID             uint      `json:"child_id" gorm:"not null"`
	SyncedAt            time.Time `json:"synced_at" gorm:"not null"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (BlueprintContentTag) TableName() string {
	return "blueprint_content_tags"
}
