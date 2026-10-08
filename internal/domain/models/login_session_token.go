package models

import "time"

// LoginSessionToken is a short-lived, single-use credential minted by
// GET /login/session_token for a caller holding an API access token. Its
// session_url, opened in a browser, signs that user into a web session
// (Canvas login/session_token). Only the SHA-256 of the token is stored;
// it is consumed with DELETE ... RETURNING. Migration 000160.
type LoginSessionToken struct {
	ID        uint   `json:"id" gorm:"column:id;primaryKey"`
	TokenHash string `json:"-" gorm:"column:token_hash;type:text;uniqueIndex;not null"`
	UserID    uint   `json:"user_id" gorm:"not null;index"`
	AccountID uint   `json:"account_id" gorm:"not null"`
	// ReturnTo is the validated same-site path the browser lands on.
	ReturnTo  string    `json:"return_to" gorm:"column:return_to;type:text;not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null;index"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName pins the table name.
func (LoginSessionToken) TableName() string { return "login_session_tokens" }
