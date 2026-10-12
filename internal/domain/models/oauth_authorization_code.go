package models

import "time"

// OAuthAuthorizationCode is a short-lived, single-use OAuth2 authorization
// code (RFC 6749 §4.1). Only the SHA-256 of the code is stored. Codes live
// in Postgres rather than process memory so that the /login/oauth2/auth
// pod and the /login/oauth2/token pod need not be the same replica (B192).
type OAuthAuthorizationCode struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	CodeHash       string    `json:"-" gorm:"column:code_hash;uniqueIndex;not null"`
	UserID         uint      `json:"user_id" gorm:"not null;index"`
	DeveloperKeyID uint      `json:"developer_key_id" gorm:"not null;index"`
	ClientID       string    `json:"client_id" gorm:"not null"`
	RedirectURI    string    `json:"redirect_uri" gorm:"column:redirect_uri;type:text;not null"`
	Scopes         string    `json:"scopes" gorm:"type:text"`
	ExpiresAt      time.Time `json:"expires_at" gorm:"not null;index"`
	CreatedAt      time.Time `json:"created_at"`
	// CodeChallenge / CodeChallengeMethod carry the RFC 7636 (PKCE)
	// challenge the client sent to /login/oauth2/auth (migration 000119).
	// NULL for a plain authorization-code request. Only "S256" is
	// accepted, as in Canvas.
	CodeChallenge       *string `json:"-" gorm:"column:code_challenge;type:text"`
	CodeChallengeMethod *string `json:"-" gorm:"column:code_challenge_method;type:text"`
}

func (OAuthAuthorizationCode) TableName() string { return "oauth_authorization_codes" }
