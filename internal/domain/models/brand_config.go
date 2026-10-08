package models

import (
	"time"

	"gorm.io/gorm"
)

// BrandConfig is an account's theme (G118; Canvas: BrandConfig + the
// Theme Editor). Variables is a JSON object of the allow-listed theme
// variables (see service.BrandVariables); the logo is stored inline so it
// can be served without a file store. Only raster image types are
// accepted for the logo (no SVG: it can carry script).
type BrandConfig struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID       uint      `json:"account_id" gorm:"not null;uniqueIndex"`
	Variables       string    `json:"variables" gorm:"type:jsonb;not null;default:'{}'"`
	LogoData        []byte    `json:"-" gorm:"type:bytea"`
	LogoContentType string    `json:"-" gorm:"not null;default:''"`
	UpdatedByUserID uint      `json:"updated_by_user_id" gorm:"not null;default:0"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// BeforeSave keeps Variables valid jsonb.
func (b *BrandConfig) BeforeSave(tx *gorm.DB) error {
	if b.Variables == "" {
		b.Variables = "{}"
	}
	return nil
}

// StorageQuota stores a storage quota in MB (G120; Canvas:
// storage_quota on courses/users/groups and the account defaults).
//
// ContextType is Course, User or Group for a per-context override, or
// AccountCourseDefault / AccountUserDefault / AccountGroupDefault for an
// account's default (ContextID = the account id). AccountID is the tenant
// the row belongs to.
type StorageQuota struct {
	ID          uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID   uint      `json:"account_id" gorm:"not null;index"`
	ContextType string    `json:"context_type" gorm:"not null;uniqueIndex:idx_storage_quotas_context,priority:1"`
	ContextID   uint      `json:"context_id" gorm:"not null;uniqueIndex:idx_storage_quotas_context,priority:2"`
	QuotaMB     int64     `json:"quota_mb" gorm:"column:quota_mb;not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName pins the plural (GORM's inflector leaves "quota" singular).
func (StorageQuota) TableName() string { return "storage_quotas" }
