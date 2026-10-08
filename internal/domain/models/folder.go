package models

import "time"

type Folder struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	ContextType    string    `json:"context_type" gorm:"not null"`
	ContextID      uint      `json:"context_id" gorm:"not null"`
	ParentFolderID *uint     `json:"parent_folder_id" gorm:"index"`
	Name           string    `json:"name" gorm:"not null"`
	FullName       string    `json:"full_name" gorm:"not null"`
	Position       int       `json:"position"`
	WorkflowState  string    `json:"workflow_state" gorm:"not null;default:'visible'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// Locked = unpublished, Hidden = left out of learner listings
	// (Canvas folders.locked / hidden; migration 000114). Both apply to
	// everything inside the folder.
	Locked bool `json:"locked" gorm:"not null;default:false"`
	Hidden bool `json:"hidden" gorm:"not null;default:false"`
}

func (Folder) TableName() string {
	return "folders"
}
