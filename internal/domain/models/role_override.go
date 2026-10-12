package models

import "time"

// RoleOverride allows per-context permission overrides for a custom role.
// For example, a role may have a permission enabled at the account level
// but overridden (disabled) for a specific course.
type RoleOverride struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID   uint      `json:"account_id" gorm:"not null;index"`
	RoleID      uint      `json:"role_id" gorm:"not null;index;uniqueIndex:idx_role_overrides_role_perm_context,priority:1"` // References CustomRole
	Permission  string    `json:"permission" gorm:"not null;uniqueIndex:idx_role_overrides_role_perm_context,priority:2"`   // Permission name constant
	Enabled     bool      `json:"enabled" gorm:"not null;default:false"`                                                    // Whether the permission is granted
	Locked      bool      `json:"locked" gorm:"not null;default:false"`                                                     // Whether a course-level override may change this
	ContextType string    `json:"context_type" gorm:"not null;default:'Account';uniqueIndex:idx_role_overrides_role_perm_context,priority:3"` // Account or Course
	ContextID   uint      `json:"context_id" gorm:"not null;default:0;uniqueIndex:idx_role_overrides_role_perm_context,priority:4"`           // ID of the context (0 = account-wide)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName ensures GORM uses the correct table name.
func (RoleOverride) TableName() string {
	return "role_overrides"
}
