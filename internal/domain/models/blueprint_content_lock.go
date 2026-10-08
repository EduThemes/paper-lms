package models

import (
	"time"

	"gorm.io/gorm"
)

// BlueprintContentLock records the lock settings of one piece of
// blueprint (source) content — Paper's equivalent of the restrictions on
// a Canvas MasterContentTag.
//
// Restrictions is a JSON object over the Canvas restriction categories
// {"content", "points", "due_dates", "availability_dates"}. A row
// overrides the template's default restrictions for that one item; an
// item with no row inherits the defaults (when the template uses them).
// A row whose restrictions are all false explicitly unlocks the item.
//
// ContentType uses the canonical names of service.contentKey:
// "Assignment", "WikiPage", "Quiz", "DiscussionTopic".
type BlueprintContentLock struct {
	ID                  uint      `json:"id" gorm:"column:id;primaryKey"`
	BlueprintTemplateID uint      `json:"blueprint_template_id" gorm:"not null;uniqueIndex:idx_bp_content_lock_item,priority:1"`
	ContentType         string    `json:"content_type" gorm:"type:text;not null;uniqueIndex:idx_bp_content_lock_item,priority:2"`
	ContentID           uint      `json:"content_id" gorm:"not null;uniqueIndex:idx_bp_content_lock_item,priority:3"`
	Restrictions        string    `json:"restrictions" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (BlueprintContentLock) TableName() string { return "blueprint_content_locks" }

// BeforeCreate mirrors the NOT NULL DEFAULT '{}' of migration 000119.
func (l *BlueprintContentLock) BeforeCreate(tx *gorm.DB) error {
	if l.Restrictions == "" {
		l.Restrictions = "{}"
	}
	return nil
}
