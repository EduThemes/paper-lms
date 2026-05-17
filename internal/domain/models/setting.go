package models

import (
	"time"

	"gorm.io/gorm"
)

// Setting is one row of the Super-Admin Settings Engine key-value store.
// Migration 000057 carries the schema; the settings service in
// internal/service/settings/ wraps reads/writes with the catalog-typed
// resolution chain (user → account-parent-chain → instance → env →
// default) and the secretbox encryption pass.
//
// Exactly one of ValuePlain / ValueEncrypted is non-nil per row — the
// settings_exactly_one_value CHECK constraint enforces this at the DB
// layer; the service layer never writes a secret value to ValuePlain
// or a non-secret value to ValueEncrypted. ValueEncrypted carries the
// secretbox-format ciphertext (1-byte key_id + 12-byte nonce + AEAD
// payload); the service decrypts on read via auth.Decrypt.
//
// FERPA export note (Wave 5 follow-up): account-scoped settings are
// part of the tenant's data export. Secrets are re-encrypted under a
// per-export key supplied by the requester rather than re-emitted in
// plaintext. See plan §"Open questions" #4.
type Setting struct {
	ID uint `json:"id" gorm:"primaryKey"`

	// ScopeType is one of 'instance', 'account', 'user'. The DB
	// settings_scope_type_check CHECK constraint enforces this.
	// 'instance' uses ScopeID=0; 'account' / 'user' use the
	// respective row's id.
	ScopeType string `json:"scope_type" gorm:"not null"`

	// ScopeID is the account_id or user_id this setting binds to.
	// Zero is reserved for instance-scope rows.
	ScopeID uint `json:"scope_id" gorm:"not null;default:0"`

	// Key is the dotted-namespace setting key declared in the
	// settings catalog (e.g. 'smtp.host', 'storage.s3.bucket').
	// Compile-time vocabulary lives in
	// internal/service/settings/catalog.go.
	Key string `json:"key" gorm:"not null"`

	// ValuePlain holds non-secret values verbatim. NULL when the row
	// stores a secret. Pointer-to-string so GORM serializes NULL
	// instead of '' — required by the settings_exactly_one_value
	// CHECK constraint, which can't tell '' from NULL.
	ValuePlain *string `json:"value_plain,omitempty"`

	// ValueEncrypted holds secretbox ciphertext (versioned key_id +
	// nonce + ciphertext+tag). NULL when the row stores a non-secret
	// value. The settings service is the only legitimate reader —
	// the API surface NEVER returns the decrypted plaintext.
	ValueEncrypted []byte `json:"-"`

	// ValueType declares how the catalog coerces the value on read
	// and which column carries it. One of
	// 'string'|'int'|'bool'|'json'|'secret' — enforced by the DB
	// CHECK constraint. 'secret' MUST pair with ValueEncrypted.
	ValueType string `json:"value_type" gorm:"not null;default:'string'"`

	// UpdatedBy is the user_id of the platform operator that last
	// wrote this row. ON DELETE SET NULL — losing the actor row
	// doesn't break the setting; the audit log carries the durable
	// who-changed-what.
	UpdatedBy *uint `json:"updated_by,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the GORM-side table name. The default ('settings')
// already matches the migration, but spelling it out is cheap and
// guards against future naming-strategy changes.
func (Setting) TableName() string {
	return "settings"
}

// BeforeCreate mirrors the SQL DEFAULTs in Go. The
// settings_value_type_check CHECK rejects ” so an empty ValueType
// must default to 'string' before INSERT — same class as Account's
// MFAPolicy / DefaultLocale defaults. Same pattern as
// CLAUDE.md "Phase 7 patterns" / [[feedback_gorm_empty_string_not_omitted]].
func (s *Setting) BeforeCreate(tx *gorm.DB) error {
	if s.ValueType == "" {
		s.ValueType = "string"
	}
	if s.ScopeType == "" {
		s.ScopeType = "instance"
	}
	return nil
}
