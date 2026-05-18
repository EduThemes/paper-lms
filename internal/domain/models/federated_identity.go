package models

import (
	"time"

	"gorm.io/datatypes"
)

// FederatedIdentity anchors an external IdP's view of a learner ("subject"
// in OIDC, NameID in SAML, principal name in CAS, DN in LDAP) to a
// local Paper LMS user row. One row per (provider, external_subject)
// pair; a single user can have many federated identities (e.g. SSO via
// Google AND a local password AND a future passkey credential).
//
// Replaces the implicit "match by email" the pre-9-PRE SAML/LDAP/CAS
// handlers do. The bug class that replaces: an IdP whose users can
// change their email could let a malicious actor claim someone else's
// Paper account by adopting their email. With this table, the bind is
// to the IdP-stable identifier, not the mutable email.
//
// ClaimsSnapshot: stores the raw claims/attributes captured at the
// FIRST login from this (provider, subject) pair. Used for the Apple
// Sign-In quirk specifically — Apple sends email + name only on the
// first consent; later logins omit them. The snapshot lets us re-use
// them without re-prompting.
type FederatedIdentity struct {
	ID              uint           `json:"id" gorm:"column:id;primaryKey"`
	UserID          uint           `json:"user_id" gorm:"not null;index"`
	ProviderID      uint           `json:"provider_id" gorm:"not null"`
	ExternalSubject string         `json:"external_subject" gorm:"not null"`
	ClaimsSnapshot  datatypes.JSON `json:"claims_snapshot,omitempty" gorm:"type:jsonb"`
	FirstSeenAt     time.Time      `json:"first_seen_at" gorm:"not null;default:now()"`
	LastSeenAt      time.Time      `json:"last_seen_at" gorm:"not null;default:now()"`
}

func (FederatedIdentity) TableName() string { return "federated_identities" }
