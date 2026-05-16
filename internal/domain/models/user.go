package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID uint `json:"id" gorm:"primaryKey"`
	// AccountID is the tenant this user belongs to. Added in migration
	// 000052 (Phase 13 / 13.1.A) — every prior user got backfilled from
	// their primary enrollment's course→account, with account 1 as the
	// legacy fallback for users with no enrollments. New users must
	// carry an account_id; handlers/auth surfaces (13.1.C-F) assert
	// this as the authoritative tenant scope.
	AccountID           uint       `json:"account_id" gorm:"not null;index"`
	Name                string     `json:"name" gorm:"not null"`
	SortableName        string     `json:"sortable_name"`
	ShortName           string     `json:"short_name"`
	LoginID             string     `json:"login_id" gorm:"uniqueIndex;not null"`
	SISUserID           *string    `json:"sis_user_id" gorm:"uniqueIndex"`
	Email               string     `json:"email" gorm:"not null"`
	PasswordHash        string     `json:"-" gorm:"not null"`
	AvatarURL           string     `json:"avatar_url"`
	Role                string     `json:"role" gorm:"not null;default:'user'"` // admin, user
	Locale              string     `json:"locale" gorm:"default:'en'"`
	TimeZone            string     `json:"time_zone" gorm:"default:'America/New_York'"`
	// LeaderboardOptOut is the user-facing privacy toggle shipped in
	// W2-C. When true, this user is excluded from public leaderboard
	// surfaces — but their currencies / awards / mastery are unchanged
	// (per SYNTHESIS §5: opting out does NOT zero progress).
	//
	// No SQL `default:` GORM tag is set on purpose: the migration
	// (000040) carries DEFAULT FALSE, and we never want this column's
	// behavior to be ambiguous if a future write path uses `db.Updates`.
	// `db.Save` (which UserRepo.Update uses) writes every column.
	LeaderboardOptOut bool `json:"leaderboard_opt_out"`

	// Phase 9-PRE — auth foundations.
	//
	// WebauthnUserHandle: 64 random bytes generated at row-insert time.
	// STABLE forever per user. Required by 9-C passkeys; populated now
	// so the future migration is zero-touch. Migration 000046 sets the
	// DEFAULT gen_random_bytes(64) so existing rows are backfilled
	// automatically.
	WebauthnUserHandle []byte `json:"-" gorm:"not null"`

	// TOTPSecretEncrypted: AES-256-GCM ciphertext of the user's TOTP
	// secret (RFC 6238). Plaintext form lives only briefly during
	// enrollment; the DB never sees it. NULL = user has not enrolled.
	// See internal/auth/secretbox.go for the ciphertext format.
	TOTPSecretEncrypted []byte `json:"-"`

	// TOTPVerifiedAt: set ONLY after the user proves they scanned the
	// QR by entering a correct 6-digit code. Enrollment is not final
	// until this timestamp is set. A stolen session that requests
	// enrollment but never verifies cannot lock the real user out.
	TOTPVerifiedAt *time.Time `json:"totp_verified_at,omitempty"`

	// TOTPLastUsedWindow (Phase 10-A.5) is the most recently consumed
	// TOTP step counter (Unix-seconds / 30). RFC 6238 §5.2 code-reuse
	// protection: a code can only be used once per window. Default 0 =
	// never used; every real-world TOTP code lands in a window > 0.
	TOTPLastUsedWindow int64 `json:"-" gorm:"not null;default:0"`

	ResetToken          string     `json:"-"`
	ResetTokenExpiresAt *time.Time `json:"-"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (u *User) HashPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hashedPassword)
	return nil
}

func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
}
