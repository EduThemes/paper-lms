package models

import (
	"time"

	"github.com/lib/pq"
)

// UserWebauthnCredential is one registered passkey for a user.
// See migration 000049 for column rationale.
//
// We store CredentialID (the WebAuthn-spec opaque identifier) as
// the lookup key for assertions; PublicKeyCOSE drives signature
// verification; SignCount is the cloned-authenticator guard.
//
// Transports is stored as a Postgres text[] (e.g. {"internal"},
// {"usb","nfc"}). pq.StringArray gives us the GORM-friendly
// adapter without an extra serializer.
type UserWebauthnCredential struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	UserID         uint           `json:"user_id" gorm:"not null;index"`
	CredentialID   []byte         `json:"-" gorm:"not null;uniqueIndex"`
	PublicKeyCOSE  []byte         `json:"-" gorm:"column:public_key_cose;not null"`
	SignCount      uint32         `json:"sign_count"`
	AAGUID         []byte         `json:"aaguid,omitempty" gorm:"column:aaguid"`
	Transports     pq.StringArray `json:"transports,omitempty" gorm:"type:text[]"`
	Nickname       string         `json:"nickname"`
	BackupEligible bool           `json:"backup_eligible"`
	BackupState    bool           `json:"backup_state"`
	LastUsedAt     *time.Time     `json:"last_used_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at" gorm:"not null;default:now()"`
}

func (UserWebauthnCredential) TableName() string { return "user_webauthn_credentials" }
