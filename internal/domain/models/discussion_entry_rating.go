package models

type DiscussionEntryRating struct {
	ID                uint `json:"id" gorm:"primaryKey"`
	DiscussionEntryID uint `json:"discussion_entry_id" gorm:"uniqueIndex:idx_rating_entry_user"`
	UserID            uint `json:"user_id" gorm:"uniqueIndex:idx_rating_entry_user"`
	Rating            int  `json:"rating" gorm:"default:1"`
}

func (DiscussionEntryRating) TableName() string {
	return "discussion_entry_ratings"
}
