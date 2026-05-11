package models

import "time"

type Folder struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	ContextType    string    `json:"context_type" gorm:"not null"`
	ContextID      uint      `json:"context_id" gorm:"not null"`
	ParentFolderID *uint     `json:"parent_folder_id" gorm:"index"`
	Name           string    `json:"name" gorm:"not null"`
	FullName       string    `json:"full_name" gorm:"not null"`
	Position       int       `json:"position"`
	WorkflowState  string    `json:"workflow_state" gorm:"not null;default:'visible'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (Folder) TableName() string {
	return "folders"
}
