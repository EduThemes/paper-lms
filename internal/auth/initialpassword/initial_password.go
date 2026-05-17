// Package initialpassword produces a cryptographically random one-time
// password for users created by automated provisioning paths — SIS
// imports, OneRoster sync, future LMS-to-LMS migrations.
//
// Why this exists: prior to this helper, the OneRoster sync used
// "OneRoster-<SourcedID>-changeme" — a deterministic string derivable
// from a public SIS identifier — and the SIS CSV import fell back to
// the static literal "changeme". Both are recoverable credentials:
// any attacker who knows a user's SIS sourcedId (or that an import
// ran) could log in as that user before the user ever touched the
// system.
//
// The fix is to never derive an initial password from anything an
// attacker can guess. The generated value here is 32 bytes of
// crypto/rand entropy, hex-encoded to 64 ASCII characters so it
// survives bcrypt's input handling unchanged. The plaintext is
// IRRECOVERABLE — the caller hashes it immediately and discards the
// plaintext.
//
// Why a sub-package of internal/auth rather than internal/auth
// itself: internal/auth imports internal/service (via auth_audit.go),
// and the SIS / OneRoster services that need this helper live in
// internal/service. A sub-package with no upstream deps sidesteps
// the cycle without restructuring the existing audit wiring. See
// internal/service/notification_delivery_service.go for the prior
// art on this cycle.
//
// IMPORTANT (operator-side contract): the user CANNOT log in with
// the generated password because the plaintext is never surfaced.
// Callers MUST arrange a separate path for the user to set a real
// password before first login — typically a password-reset email,
// or an SSO-only account. If/when a "force password reset on first
// login" flag lands on the User model, set it to true at the same
// call site so the login surface refuses to mint a session until
// reset completes. As of 2026-05-17 that flag does NOT exist on
// models.User — see the PR body for follow-up.
package initialpassword

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

// GenerateInitialPassword returns 32 bytes of crypto/rand entropy
// hex-encoded as a 64-character string. The output is suitable to
// pass directly to (*models.User).HashPassword.
//
// The generated value is irrecoverable; users MUST be forced to
// reset on first login. Set User.RequiresPasswordReset = true (or
// equivalent flag) when calling this — see the package doc comment
// for the operator-side contract.
func GenerateInitialPassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
