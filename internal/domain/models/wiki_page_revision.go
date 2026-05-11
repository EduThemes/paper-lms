package models

import "time"

type WikiPageRevision struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	WikiPageID     uint      `gorm:"index;not null" json:"wiki_page_id"`
	RevisionNumber int       `gorm:"not null" json:"revision_number"`
	Title          string    `gorm:"type:varchar(255);not null" json:"title"`
	Body           string    `gorm:"type:text" json:"body"`
	EditedBy       uint      `gorm:"not null" json:"edited_by"`
	CreatedAt      time.Time `json:"created_at"`
}
