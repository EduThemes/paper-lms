package models

import "time"

type AnnouncementReadReceipt struct {
	ID             uint       `json:"id" gorm:"column:id;primaryKey"`
	AnnouncementID uint       `json:"announcement_id" gorm:"uniqueIndex:idx_announcement_user"`
	UserID         uint       `json:"user_id" gorm:"uniqueIndex:idx_announcement_user"`
	ReadAt         time.Time  `json:"read_at"`
	Acknowledged   bool       `json:"acknowledged" gorm:"default:false"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
}
