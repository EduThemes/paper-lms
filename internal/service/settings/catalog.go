// Package settings is the Super-Admin Settings Engine.
//
// The catalog is the compile-time vocabulary of every promotable
// operational setting. Each Definition declares the key, the type the
// service coerces the value to, the scopes the setting can be bound
// at, the env-var fallback name (so env-var-driven deployments keep
// working), the hard-coded default if neither setting nor env is set,
// and the optional TestAction tag the UI wires to the
// /superadmin/settings/test/{action} endpoints (Wave 3).
//
// Wave 1 is the storage + resolution layer; the vocabulary endpoint
// that serves Definitions to the frontend lands in Wave 2.
//
// Catalog entries cover every env var that is "promote-to-setting
// reasonable" per the plan §"What env vars currently exist". The five
// bootstrap-critical env vars (JWT_SECRET, MFA_ENCRYPTION_KEY,
// DATABASE_URL, ENVIRONMENT, FRONTEND_URL) are intentionally NOT in
// the catalog — promoting them would create a chicken-and-egg with
// the settings store itself. See plan §"Known risks".
package settings

// ValueType declares how the catalog coerces the value on read and
// which storage column carries it. Mirrors the
// settings_value_type_check CHECK constraint in migration 000057.
type ValueType string

const (
	TypeString ValueType = "string"
	TypeInt    ValueType = "int"
	TypeBool   ValueType = "bool"
	TypeJSON   ValueType = "json"
	TypeSecret ValueType = "secret"
)

// ScopeType is one of the three resolution scopes. Mirrors the
// settings_scope_type_check CHECK constraint.
type ScopeType string

const (
	ScopeInstance ScopeType = "instance"
	ScopeAccount  ScopeType = "account"
	ScopeUser     ScopeType = "user"
)

// Definition is one row of the catalog — the compile-time declaration
// of a single settable key. The vocabulary endpoint (Wave 2) returns
// a stripped serialization of these so the frontend can build the
// settings UI without hardcoding form schemas.
type Definition struct {
	Key         string      // dotted-namespace, e.g. "smtp.host"
	Group       string      // UI grouping label, e.g. "Email"
	Label       string      // human-readable input label
	Description string      // help text shown beside the input
	ValueType   ValueType   // how to coerce on read / which column to store
	Scopes      []ScopeType // scopes at which this key may be set
	EnvFallback string      // env var name to read if no setting row exists
	Default     string      // hard-coded fallback if neither setting nor env
	TestAction  string      // "email" | "s3" | "oidc" | "anthropic" | "" (no test)
}

// AllowsScope returns true when this catalog entry may be set at the
// given scope. The service rejects writes at any non-allowed scope.
func (d Definition) AllowsScope(s ScopeType) bool {
	for _, allowed := range d.Scopes {
		if allowed == s {
			return true
		}
	}
	return false
}

// IsSecret returns true for secret-valued definitions. Secret values
// route through auth.Encrypt on write and auth.Decrypt on read; the
// API surface NEVER echoes plaintext for these keys.
func (d Definition) IsSecret() bool {
	return d.ValueType == TypeSecret
}

// Catalog is the master list of promotable settings. Append new keys
// here; the rest of the system (frontend vocabulary endpoint, service
// validator, env-fallback path) picks them up automatically.
//
// The five hard-excluded bootstrap-critical env vars (JWT_SECRET,
// MFA_ENCRYPTION_KEY, DATABASE_URL, ENVIRONMENT, FRONTEND_URL) do NOT
// appear here. Promoting any of them would mean the settings store
// can't bootstrap itself.
var Catalog = []Definition{
	// ── Email ───────────────────────────────────────────────────────
	{
		Key: "smtp.host", Group: "Email", Label: "SMTP host",
		Description: "Hostname of the outbound SMTP server.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "SMTP_HOST",
		TestAction:  "email",
	},
	{
		Key: "smtp.port", Group: "Email", Label: "SMTP port",
		Description: "TCP port for the SMTP submission server. 587 for STARTTLS, 465 for implicit TLS.",
		ValueType:   TypeInt,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "SMTP_PORT",
		Default:     "587",
	},
	{
		Key: "smtp.username", Group: "Email", Label: "SMTP username",
		Description: "Authentication username for the SMTP server.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "SMTP_USERNAME",
	},
	{
		Key: "smtp.password", Group: "Email", Label: "SMTP password",
		Description: "Authentication password for the SMTP server. Stored encrypted; the API never returns the plaintext.",
		ValueType:   TypeSecret,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "SMTP_PASSWORD",
	},
	{
		Key: "smtp.from", Group: "Email", Label: "From address",
		Description: "Envelope-from address on outbound system mail.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "SMTP_FROM",
		Default:     "noreply@paperlms.org",
	},
	{
		Key: "smtp.enabled", Group: "Email", Label: "Send mail",
		Description: "Master switch — when off, email delivery is suppressed even if SMTP creds are configured.",
		ValueType:   TypeBool,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "SMTP_ENABLED",
		Default:     "false",
	},

	// ── File storage ────────────────────────────────────────────────
	{
		Key: "storage.backend", Group: "File storage", Label: "Storage backend",
		Description: "Where uploaded files land. 'local' uses the server's filesystem; 's3' targets an S3-compatible bucket (AWS, Cloudflare R2, MinIO).",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "STORAGE_BACKEND",
		Default:     "local",
	},
	{
		Key: "storage.s3.bucket", Group: "File storage", Label: "S3 bucket",
		Description: "Name of the S3-compatible bucket files are written to.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "S3_BUCKET",
		TestAction:  "s3",
	},
	{
		Key: "storage.s3.region", Group: "File storage", Label: "S3 region",
		Description: "AWS region of the bucket. Leave default for non-AWS providers that ignore region.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "S3_REGION",
		Default:     "us-east-1",
	},
	{
		Key: "storage.s3.endpoint", Group: "File storage", Label: "S3 endpoint",
		Description: "Custom endpoint URL for non-AWS S3-compatible services (Cloudflare R2, MinIO). Leave empty for AWS.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "S3_ENDPOINT",
	},
	{
		Key: "storage.s3.access_key", Group: "File storage", Label: "S3 access key",
		Description: "Access key ID for the bucket. Stored encrypted.",
		ValueType:   TypeSecret,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "S3_ACCESS_KEY",
	},
	{
		Key: "storage.s3.secret_key", Group: "File storage", Label: "S3 secret key",
		Description: "Secret access key for the bucket. Stored encrypted.",
		ValueType:   TypeSecret,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "S3_SECRET_KEY",
	},

	// ── AI ──────────────────────────────────────────────────────────
	{
		Key: "ai.anthropic.api_key", Group: "AI (Anthropic)", Label: "Anthropic API key",
		Description: "API key for the Claude Messages API. Required for AI Assist features in the rich editor.",
		ValueType:   TypeSecret,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "ANTHROPIC_API_KEY",
		TestAction:  "anthropic",
	},

	// ── Federated auth (SAML defaults) ──────────────────────────────
	{
		Key: "auth.saml.entity_id", Group: "Federated auth", Label: "SAML entity ID",
		Description: "Service Provider entity ID used in SAML AuthnRequests. Usually matches your deployment URL.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "SAML_ENTITY_ID",
	},
	{
		Key: "auth.saml.cert_file", Group: "Federated auth", Label: "SAML cert path",
		Description: "Filesystem path to the SP signing certificate (PEM). Filesystem-based for v1; Wave 5 may promote to inline secret.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "SAML_CERT_FILE",
	},
	{
		Key: "auth.saml.key_file", Group: "Federated auth", Label: "SAML key path",
		Description: "Filesystem path to the SP signing key (PEM). Filesystem-based for v1; Wave 5 may promote to inline secret.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "SAML_KEY_FILE",
	},

	// ── Federated auth (OIDC redirect base) ─────────────────────────
	{
		Key: "auth.oidc.redirect_base", Group: "Federated auth", Label: "OIDC redirect base URL",
		Description: "Base URL used to build the OIDC callback redirect_uri. Defaults to the deployment's Frontend URL.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "OIDC_REDIRECT_BASE",
		TestAction:  "oidc",
	},

	// ── Passkeys ────────────────────────────────────────────────────
	{
		Key: "auth.passkey.rpid", Group: "Passkeys", Label: "WebAuthn RP ID",
		Description: "Relying Party ID for WebAuthn — the bare domain of the deployment (no scheme, no port). " +
			"WARNING: changing this invalidates EVERY existing passkey on the deployment. The RPID is hashed into each enrolled credential and cannot be rotated without re-enrollment. " +
			"Must be a registrable suffix of every entry in WebAuthn RP origins; setting it too broad (e.g. 'example.edu' when origin is 'lms.example.edu') would let other subdomains complete ceremonies with this deployment's credentials.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "PASSKEY_RPID",
		Default:     "localhost",
	},
	{
		Key: "auth.passkey.rporigins", Group: "Passkeys", Label: "WebAuthn RP origins",
		Description: "Comma-separated list of allowed origins for passkey ceremonies. Defaults to the Frontend URL.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance},
		EnvFallback: "PASSKEY_RPORIGINS",
	},

	// ── Branding ────────────────────────────────────────────────────
	{
		Key: "branding.frontend_url", Group: "Branding", Label: "Frontend URL",
		Description: "Public URL where the SPA is served. Used as the default origin for password reset links, OIDC callbacks, and passkey ceremonies.",
		ValueType:   TypeString,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "FRONTEND_URL",
		Default:     "http://localhost:5173",
	},

	// ── Quotas & limits ─────────────────────────────────────────────
	{
		Key: "quotas.max_upload_size_mb", Group: "Quotas & limits", Label: "Max upload size (MB)",
		Description: "Per-request file upload cap. Account-scoped overrides take precedence over the instance default. Reconciles with accounts.max_upload_size_mb (account-row column wins when set).",
		ValueType:   TypeInt,
		Scopes:      []ScopeType{ScopeInstance, ScopeAccount},
		EnvFallback: "MAX_UPLOAD_SIZE_MB",
		Default:     "500",
	},
}

// Find returns the catalog entry for the given key, or (zero, false)
// if the key is not declared. The service rejects all reads/writes
// for unknown keys — stringly-typed callers can't accidentally write
// to "smpt.host" and discover the typo at runtime.
func Find(key string) (Definition, bool) {
	for _, d := range Catalog {
		if d.Key == key {
			return d, true
		}
	}
	return Definition{}, false
}
