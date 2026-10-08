package models

import "time"

// WikiPageRevision is one saved version of a wiki page (Canvas: the
// page's simply_versioned history). Every save that changes the title or
// body appends a row; RevisionNumber counts up from 1 per page. A revert
// copies an old revision back onto the page and records that as a new
// revision, so history is never rewritten.
//
// EditedBy is nullable: the 000113 backfill seeds revision 1 for every
// existing page with no known author.
type WikiPageRevision struct {
	ID             uint      `gorm:"column:id;primaryKey" json:"id"`
	WikiPageID     uint      `gorm:"not null;uniqueIndex:idx_wiki_page_revisions_page_number,priority:1" json:"wiki_page_id"`
	RevisionNumber int       `gorm:"not null;uniqueIndex:idx_wiki_page_revisions_page_number,priority:2" json:"revision_number"`
	Title          string    `gorm:"type:text;not null" json:"title"`
	Body           string    `gorm:"type:text" json:"body"`
	EditedBy       *uint     `gorm:"index" json:"edited_by"`
	CreatedAt      time.Time `json:"created_at"`
}

func (WikiPageRevision) TableName() string {
	return "wiki_page_revisions"
}
