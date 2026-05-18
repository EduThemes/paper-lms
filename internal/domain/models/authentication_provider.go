package models

import (
	"time"

	"github.com/lib/pq"
)

type AuthenticationProvider struct {
	ID        uint   `json:"id" gorm:"column:id;primaryKey"`
	AccountID uint   `json:"account_id" gorm:"index;not null"`
	AuthType  string `json:"auth_type" gorm:"not null"` // "saml", "ldap", "cas", "oidc"
	Position  int    `json:"position" gorm:"default:1"`

	// SAML settings
	IDPEntityID            string `json:"idp_entity_id,omitempty" gorm:"column:id_p_entity_id"`
	LogInURL               string `json:"log_in_url,omitempty"`
	LogOutURL              string `json:"log_out_url,omitempty"`
	CertificateFingerprint string `json:"certificate_fingerprint,omitempty"`
	IDPCertificate         string `json:"idp_certificate,omitempty" gorm:"column:id_p_certificate;type:text"` // PEM or base64-encoded X.509 cert for signature verification

	// LDAP settings
	LDAPHost   string `json:"ldap_host,omitempty" gorm:"column:ldap_host"`
	LDAPPort   int    `json:"ldap_port,omitempty" gorm:"column:ldap_port"`
	LDAPBase   string `json:"ldap_base,omitempty" gorm:"column:ldap_base"`
	LDAPFilter string `json:"ldap_filter,omitempty" gorm:"column:ldap_filter"`
	LDAPBindDN string `json:"ldap_bind_dn,omitempty" gorm:"column:ldap_bind_dn"`
	// LDAPBindPassword is the legacy plaintext bind password column.
	// Phase 9-PRE moved the canonical write to LDAPBindPasswordEncrypted
	// (AES-256-GCM via internal/auth/secretbox). The Create/Update
	// handlers now seal new passwords into the encrypted column and
	// blank this field; the LDAP read path (resolveLDAPBindPassword in
	// internal/auth/ldap.go) prefers the encrypted column and falls
	// back to this one only for un-rotated rows.
	//
	// TODO: drop in Wave-B follow-up migration after the Go-side
	// backfill (RebackfillLDAPBindPasswords at boot) finishes flipping
	// every plaintext row to ciphertext. At that point both this field
	// and the SQL column go away.
	LDAPBindPassword   string `json:"-" gorm:"column:ldap_bind_password"` // Never expose in JSON
	LDAPUseTLS         bool   `json:"ldap_use_tls" gorm:"column:ldap_use_tls"`
	LDAPLoginAttribute string `json:"ldap_login_attribute,omitempty" gorm:"column:ldap_login_attribute;default:'uid'"`

	// CAS settings
	CASBaseURL     string `json:"cas_base_url,omitempty" gorm:"column:cas_base_url"`
	CASLoginURL    string `json:"cas_login_url,omitempty" gorm:"column:cas_login_url"`
	CASValidateURL string `json:"cas_validate_url,omitempty" gorm:"column:cas_validate_url"`
	CASLogoutURL   string `json:"cas_logout_url,omitempty" gorm:"column:cas_logout_url"`

	// General settings
	JITProvisioning     bool              `json:"jit_provisioning" gorm:"column:jit_provisioning;default:false"`
	FederatedAttributes map[string]string `json:"federated_attributes,omitempty" gorm:"serializer:json"`
	WorkflowState       string            `json:"workflow_state" gorm:"default:'active'"`

	// Phase 9-PRE additions.
	//
	// LDAPBindPasswordEncrypted: AES-256-GCM ciphertext of the LDAP
	// service-account password. Replaces the plaintext LDAPBindPassword
	// field (which stays for one release while the Go-side backfill
	// migrates existing rows on first boot, then drops in 000048).
	LDAPBindPasswordEncrypted []byte `json:"-" gorm:"column:ldap_bind_password_encrypted"`

	// AutoProvision: per-provider JIT toggle. Default FALSE; the repo
	// layer flips it to TRUE for the FIRST provider an admin creates
	// for a tenant (user decision 2026-05-15). The legacy
	// JITProvisioning field above is read-compatible with this new
	// one — `auto_provision || jit_provisioning` is treated as
	// "JIT enabled" during the deprecation window.
	AutoProvision bool `json:"auto_provision"`

	// OIDC settings (Sprint 9-A).
	//
	// OIDCIssuerURL is the IdP's OpenID Connect discovery base — the
	// coreos/go-oidc library reads /.well-known/openid-configuration
	// from this URL to discover the authorization, token, jwks, and
	// userinfo endpoints. Examples:
	//   * Google Workspace:    https://accounts.google.com
	//   * Microsoft Entra ID:  https://login.microsoftonline.com/{tenant}/v2.0
	//   * Apple Sign-In:       https://appleid.apple.com
	//   * Generic OIDC:        whatever the admin supplies
	// gorm `column:` tags are load-bearing — GORM's PascalCase-to-
	// snake_case naming strategy converts `OIDCIssuerURL` to
	// `o_id_c_issuer_url`, which doesn't match the migration column
	// names (`oidc_issuer_url`). Without explicit `column:` tags,
	// GORM INSERT fails with `column "o_id_c_issuer_url" does not exist`.
	OIDCIssuerURL             string         `json:"oidc_issuer_url,omitempty" gorm:"column:oidc_issuer_url"`
	OIDCClientID              string         `json:"oidc_client_id,omitempty" gorm:"column:oidc_client_id"`
	OIDCClientSecretEncrypted []byte         `json:"-" gorm:"column:oidc_client_secret_encrypted"` // AES-256-GCM via internal/auth/secretbox
	OIDCScopes                pq.StringArray `json:"oidc_scopes,omitempty" gorm:"type:text[];column:oidc_scopes"`
	OIDCPreset                string         `json:"oidc_preset,omitempty" gorm:"column:oidc_preset"` // "google" | "microsoft" | "apple" | "generic"

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
