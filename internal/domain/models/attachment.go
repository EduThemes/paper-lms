package models

import "time"

type Attachment struct {
	ID            uint               `json:"id" gorm:"column:id;primaryKey"`
	ContextType   string             `json:"context_type" gorm:"not null"`
	ContextID     uint               `json:"context_id" gorm:"not null"`
	FolderID      *uint              `json:"folder_id" gorm:"index"`
	UserID        uint               `json:"user_id" gorm:"not null"`
	DisplayName   string             `json:"display_name" gorm:"not null"`
	Filename      string             `json:"filename" gorm:"not null"`
	ContentType   string             `json:"content_type" gorm:"not null"`
	Size          int64              `json:"size" gorm:"not null"`
	MD5           string             `json:"md5" gorm:"column:md5"`
	StoragePath   string             `json:"-" gorm:"not null"`
	WorkflowState AttachmentWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'active'"`
	FileState     string             `json:"file_state" gorm:"not null;default:'available'"`
	UploadStatus  string             `json:"upload_status" gorm:"not null;default:'success'"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`

	// File visibility (Canvas attachments.locked / hidden / lock_at /
	// unlock_at; migration 000095). Locked = unpublished: learners can
	// neither list nor download it. Hidden = "only available with link":
	// left out of learner listings but downloadable by id. LockAt /
	// UnlockAt bound the window in which learners may read the file.
	Locked   bool       `json:"locked" gorm:"not null;default:false"`
	Hidden   bool       `json:"hidden" gorm:"not null;default:false"`
	LockAt   *time.Time `json:"lock_at"`
	UnlockAt *time.Time `json:"unlock_at"`

	// Usage rights (Canvas UsageRights, inlined). Justification is one of
	// the UsageRights* consts; empty means "not set".
	UsageRightsJustification  string `json:"usage_rights_justification" gorm:"type:text;not null;default:''"`
	UsageRightsLegalCopyright string `json:"usage_rights_legal_copyright" gorm:"type:text;not null;default:''"`
	UsageRightsLicense        string `json:"usage_rights_license" gorm:"type:text;not null;default:''"`
}

func (Attachment) TableName() string {
	return "attachments"
}
