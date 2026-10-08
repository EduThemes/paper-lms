package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"golang.org/x/crypto/bcrypt"
)

// UserLogin is an additional login (Canvas: pseudonym) for a user (G115).
// The user's primary login stays on users.login_id; extra logins let one
// person sign in with a second local username, carry a second SIS id, or
// be matched by an SSO provider's subject (e.g. an LDAP uid or a CAS
// user) that differs from their email.
//
//   - AuthenticationProviderID nil: a local login. It authenticates with
//     its own password when one is set, otherwise with the user's.
//   - AuthenticationProviderID set: the login is the identity that
//     provider asserts. The LoginPipeline matches the provider's subject
//     against UniqueID (inside the provider's account) before falling back
//     to email auto-link.
type UserLogin struct {
	ID        uint `json:"id" gorm:"column:id;primaryKey"`
	UserID    uint `json:"user_id" gorm:"not null;index"`
	AccountID uint `json:"account_id" gorm:"not null;index"`
	// UniqueID is the login name as entered; UniqueIDNormalized is its
	// lower-cased form, unique among active logins (deployment-wide, like
	// users.login_id, because local sign-in happens before a tenant is
	// known).
	UniqueID           string  `json:"unique_id" gorm:"column:unique_id;not null"`
	UniqueIDNormalized string  `json:"-" gorm:"column:unique_id_normalized;not null;uniqueIndex:idx_user_logins_unique_id_active,where:workflow_state = 'active'"`
	SISUserID          *string `json:"sis_user_id" gorm:"column:sis_user_id;index"`
	// AuthenticationProviderID ties the login to one SSO provider of the
	// same account; nil = local.
	AuthenticationProviderID *uint      `json:"authentication_provider_id" gorm:"column:authentication_provider_id;index"`
	PasswordHash             string     `json:"-" gorm:"not null;default:''"`
	WorkflowState            string     `json:"workflow_state" gorm:"not null;default:'active'"` // active, suspended, deleted
	LastLoginAt              *time.Time `json:"last_login_at"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

// UserLogin workflow states.
const (
	UserLoginActive    = "active"
	UserLoginSuspended = "suspended"
	UserLoginDeleted   = "deleted"
)

// NormalizeLoginID lower-cases and trims a login name for uniqueness and
// lookup.
func NormalizeLoginID(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// BeforeSave keeps the normalized column in step and mirrors the SQL
// DEFAULT on workflow_state (GORM writes '' for a zero string).
func (l *UserLogin) BeforeSave(tx *gorm.DB) error {
	l.UniqueID = strings.TrimSpace(l.UniqueID)
	l.UniqueIDNormalized = NormalizeLoginID(l.UniqueID)
	if l.WorkflowState == "" {
		l.WorkflowState = UserLoginActive
	}
	return nil
}

// HasPassword reports whether the login carries its own password.
func (l *UserLogin) HasPassword() bool { return l.PasswordHash != "" }

// SetPassword hashes and stores a login-specific password.
func (l *UserLogin) SetPassword(password string) error {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	l.PasswordHash = string(h)
	return nil
}

// CheckPassword verifies a login-specific password.
func (l *UserLogin) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(l.PasswordHash), []byte(password))
}
