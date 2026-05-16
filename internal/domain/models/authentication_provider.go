package models

import (
	"time"

	"github.com/lib/pq"
)

type AuthenticationProvider struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	AccountID uint   `json:"account_id" gorm:"index;not null"`
	AuthType  string `json:"auth_type" gorm:"not null"` // "saml", "ldap", "cas", "oidc"
	Position  int    `json:"position" gorm:"default:1"`

	// SAML settings
	IDPEntityID            string `json:"idp_entity_id,omitempty"`
	LogInURL               string `json:"log_in_url,omitempty"`
	LogOutURL              string `json:"log_out_url,omitempty"`
	CertificateFingerprint string `json:"certificate_fingerprint,omitempty"`
	IDPCertificate         string `json:"idp_certificate,omitempty" gorm:"type:text"` // PEM or base64-encoded X.509 cert for signature verification

	// LDAP settings
	LDAPHost           string `json:"ldap_host,omitempty"`
	LDAPPort           int    `json:"ldap_port,omitempty"`
	LDAPBase           string `json:"ldap_base,omitempty"`
	LDAPFilter         string `json:"ldap_filter,omitempty"`
	LDAPBindDN         string `json:"ldap_bind_dn,omitempty"`
	LDAPBindPassword   string `json:"-"` // Never expose in JSON
	LDAPUseTLS         bool   `json:"ldap_use_tls"`
	LDAPLoginAttribute string `json:"ldap_login_attribute,omitempty" gorm:"default:'uid'"`

	// CAS settings
	CASBaseURL     string `json:"cas_base_url,omitempty"`
	CASLoginURL    string `json:"cas_login_url,omitempty"`
	CASValidateURL string `json:"cas_validate_url,omitempty"`
	CASLogoutURL   string `json:"cas_logout_url,omitempty"`

	// General settings
	JITProvisioning     bool              `json:"jit_provisioning" gorm:"default:false"`
	FederatedAttributes map[string]string `json:"federated_attributes,omitempty" gorm:"serializer:json"`
	WorkflowState       string            `json:"workflow_state" gorm:"default:'active'"`

	// Phase 9-PRE additions.
	//
	// LDAPBindPasswordEncrypted: AES-256-GCM ciphertext of the LDAP
	// service-account password. Replaces the plaintext LDAPBindPassword
	// field (which stays for one release while the Go-side backfill
	// migrates existing rows on first boot, then drops in 000048).
	LDAPBindPasswordEncrypted []byte `json:"-"`

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
	OIDCIssuerURL              string         `json:"oidc_issuer_url,omitempty"`
	OIDCClientID               string         `json:"oidc_client_id,omitempty"`
	OIDCClientSecretEncrypted  []byte         `json:"-"` // AES-256-GCM via internal/auth/secretbox
	OIDCScopes                 pq.StringArray `json:"oidc_scopes,omitempty" gorm:"type:text[]"`
	OIDCPreset                 string         `json:"oidc_preset,omitempty"` // "google" | "microsoft" | "apple" | "generic"

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
