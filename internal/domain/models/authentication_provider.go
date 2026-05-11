package models

import "time"

type AuthenticationProvider struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	AccountID uint   `json:"account_id" gorm:"index;not null"`
	AuthType  string `json:"auth_type" gorm:"not null"` // "saml", "ldap", "cas"
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

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
