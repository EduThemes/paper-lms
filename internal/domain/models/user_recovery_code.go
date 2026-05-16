package models

import "time"

// UserRecoveryCode is one single-use recovery code for TOTP step-up
// fallback. Stored as a bcrypt hash; the plaintext is shown to the
// user exactly once at enrollment. Use marks the row consumed
// (sets used_at) — the partial index in migration 000046 only covers
// WHERE used_at IS NULL, so used codes don't pollute the lookup path.
//
// Why not "regenerate every login": each user gets RecoveryCodeCount
// (10) codes at enrollment, shown once, expected to be saved
// somewhere safe (password manager, printed sheet in a desk drawer).
// Rotating them silently would invalidate the saved copies and lock
// users out — the exact failure mode recovery codes are supposed to
// prevent.
type UserRecoveryCode struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	UserID    uint       `json:"user_id" gorm:"not null;index"`
	CodeHash  string     `json:"-" gorm:"not null"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at" gorm:"not null;default:now()"`
}

func (UserRecoveryCode) TableName() string { return "user_recovery_codes" }
