package models

import "time"

type PageView struct {
	ID                 uint   `json:"id" gorm:"column:id;primaryKey"`
	UserID             uint   `json:"user_id" gorm:"not null;index"`
	ContextType        string `json:"context_type" gorm:"not null"`
	ContextID          uint   `json:"context_id" gorm:"not null;index"`
	URL                string `json:"url" gorm:"column:url"`
	Action             string `json:"action"`
	Participated       bool   `json:"participated" gorm:"default:false"`
	InteractionSeconds int    `json:"interaction_seconds" gorm:"default:0"`
	// AssetCategory is the kind of course content viewed (assignments,
	// discussions, pages, quizzes, files, modules, ...) — the grouping
	// course analytics reports page views by. '' when unknown.
	AssetCategory string    `json:"asset_category" gorm:"column:asset_category;type:text;not null;default:''"`
	CreatedAt     time.Time `json:"created_at"`
}
