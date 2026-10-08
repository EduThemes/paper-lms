package models

import "time"

// ContextExternalTool is an LTI tool installed (deployed) in a course or a
// tenant account. An LTI 1.3 install points at an LTI developer key and
// its tool configuration; an LTI 1.1 install has DeveloperKeyID 0 and
// carries its own launch URL, consumer key and (sealed) shared secret.
// The install's ID is the LTI 1.3 deployment_id.
type ContextExternalTool struct {
	ID             uint   `json:"id" gorm:"column:id;primaryKey"`
	ContextType    string `json:"context_type" gorm:"not null"` // Course, Account
	ContextID      uint   `json:"context_id" gorm:"not null;index"`
	DeveloperKeyID uint   `json:"developer_key_id" gorm:"not null;index"`
	Name           string `json:"name" gorm:"not null"`
	Description    string `json:"description"`
	URL            string `json:"url" gorm:"column:url"`
	Domain         string `json:"domain"`
	ConsumerKey    string `json:"consumer_key"` // For LTI 1.1 backward compat
	SharedSecret   string `json:"-"`            // For LTI 1.1 backward compat
	CustomFields   string `json:"custom_fields" gorm:"type:text"`
	WorkflowState  string `json:"workflow_state" gorm:"default:'active'"`
	// PrivacyLevel governs the PII an LTI 1.1 launch sends (anonymous,
	// name_only, email_only, public). LTI 1.3 installs use the tool
	// configuration's privacy level instead.
	PrivacyLevel string    `json:"privacy_level" gorm:"column:privacy_level;default:'anonymous'"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// No DeveloperKey association: an LTI 1.1 install has
	// developer_key_id 0, which a GORM-generated foreign key (dev
	// AutoMigrate) would reject. The SQL chain never had that FK.
}

// IsLTI11 reports whether the install is an LTI 1.1 (OAuth1-signed) tool.
func (t *ContextExternalTool) IsLTI11() bool {
	return t != nil && t.DeveloperKeyID == 0 && t.ConsumerKey != ""
}
