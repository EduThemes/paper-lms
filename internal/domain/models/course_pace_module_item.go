package models

import "time"

type CoursePaceModuleItem struct {
	ID           uint      `json:"id" gorm:"column:id;primaryKey"`
	CoursePaceID uint      `json:"course_pace_id" gorm:"not null;uniqueIndex:idx_pace_module_item"`
	ModuleItemID uint      `json:"module_item_id" gorm:"not null;uniqueIndex:idx_pace_module_item"`
	Duration     int       `json:"duration" gorm:"not null;default:1"` // days
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
