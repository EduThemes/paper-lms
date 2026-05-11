package models

import "time"

type LTIToolConfiguration struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	DeveloperKeyID    uint      `json:"developer_key_id" gorm:"uniqueIndex;not null"`
	Title             string    `json:"title" gorm:"not null"`
	Description       string    `json:"description"`
	TargetLinkURI     string    `json:"target_link_uri" gorm:"not null"`
	OIDCInitiationURL string    `json:"oidc_initiation_url" gorm:"not null"`
	Domain            string    `json:"domain"`
	ToolID            string    `json:"tool_id"`                                          // Tool-provided identifier
	PrivacyLevel      string    `json:"privacy_level" gorm:"default:'anonymous'"`         // anonymous, name_only, email_only, public
	PublicJWKURL      string    `json:"public_jwk_url"`                                   // Tool's JWKS endpoint
	PublicJWK         string    `json:"public_jwk" gorm:"type:text"`                      // Tool's public key as JSON
	CustomFields      string    `json:"custom_fields" gorm:"type:text"`                   // JSON object
	Placements        string    `json:"placements" gorm:"type:text"`                      // JSON array of placement configs
	Scopes            string    `json:"scopes" gorm:"type:text"`                          // JSON array of LTI scopes
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	// Associations
	DeveloperKey DeveloperKey `json:"-" gorm:"foreignKey:DeveloperKeyID"`
}
