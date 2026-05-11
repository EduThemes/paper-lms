package models

import "time"

type Attachment struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	ContextType   string    `json:"context_type" gorm:"not null"`
	ContextID     uint      `json:"context_id" gorm:"not null"`
	FolderID      *uint     `json:"folder_id" gorm:"index"`
	UserID        uint      `json:"user_id" gorm:"not null"`
	DisplayName   string    `json:"display_name" gorm:"not null"`
	Filename      string    `json:"filename" gorm:"not null"`
	ContentType   string    `json:"content_type" gorm:"not null"`
	Size          int64     `json:"size" gorm:"not null"`
	MD5           string    `json:"md5"`
	StoragePath   string    `json:"-" gorm:"not null"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'active'"`
	FileState     string    `json:"file_state" gorm:"not null;default:'available'"`
	UploadStatus  string    `json:"upload_status" gorm:"not null;default:'success'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Attachment) TableName() string {
	return "attachments"
}
