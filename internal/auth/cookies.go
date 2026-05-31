package auth

import "os"

// SecureCookies reports whether auth/session cookies should carry the Secure
// flag. It mirrors the local-login path (handlers.UserHandler gates on
// environment == "production"): in production the app is served over HTTPS, so
// the session cookie minted on every credential path — local, OIDC, SAML, CAS,
// LDAP, MFA step-up, passkey — must be Secure to prevent interception over a
// downgraded/MITM'd connection. Driven by the same ENVIRONMENT variable the
// config package reads at boot, so development (plain HTTP) keeps working.
func SecureCookies() bool {
	return os.Getenv("ENVIRONMENT") == "production"
}
