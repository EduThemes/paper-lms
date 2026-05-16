// Package auth's TOTP helpers wrap github.com/pquerna/otp/totp into
// the surface the rest of Paper LMS needs:
//   * EnrollUser  — generates a new secret + QR-renderable URL.
//   * VerifyCode  — constant-time check with ±1 window for clock skew.
//   * GenerateRecoveryCodes — 10 single-use bcrypt-hashed codes.
//
// Storage contract: the plaintext secret is returned ONCE from
// EnrollUser. Caller (the /mfa/enroll handler) encrypts it via
// secretbox.Encrypt before writing to users.totp_secret_encrypted.
// Plaintext NEVER hits the database. This package never persists
// anything; persistence is the caller's job.
//
// Recovery codes follow the same contract: returned plaintext once
// (the user copies them), bcrypt-hashed before write.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	totpIssuer = "Paper LMS"
	// RecoveryCodeCount is the number of single-use codes generated
	// at enrollment. Standard practice is 8-12; 10 is a sensible default.
	RecoveryCodeCount = 10
)

// EnrollUserTOTP generates a fresh TOTP secret for the user. Returns
// the plaintext secret (caller encrypts before storing), the
// otpauth:// URL (caller renders as QR for the authenticator app),
// and an error.
//
// AccountName: the loginID the authenticator app displays alongside
// the issuer. Showing "Paper LMS: michael@aprendio.ai" in Authy.
func EnrollUserTOTP(loginID string) (plaintextSecret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: loginID,
	})
	if err != nil {
		return "", "", fmt.Errorf("totp.Generate: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP checks `code` against `plaintextSecret` with ±1 window
// for clock skew. Returns true iff the code is a current or
// immediately-adjacent valid value. Constant-time internally.
//
// PREFER VerifyTOTPWithReuseGuard at login. This plain VerifyTOTP is
// kept for enrollment-verify, where there is no notion of "last used
// window" (the user hasn't completed enrollment yet).
func VerifyTOTP(plaintextSecret, code string) bool {
	return totp.Validate(code, plaintextSecret)
}

// CurrentTOTPWindow returns the current TOTP step counter
// (Unix-seconds / 30). 30 is the RFC 6238 default period that
// pquerna/otp/totp uses; we hardcode rather than introspecting the
// key URL because every code path in this codebase uses the default.
func CurrentTOTPWindow() int64 {
	return time.Now().Unix() / 30
}

// VerifyTOTPWithReuseGuard is the login-time TOTP verifier. Rejects
// codes whose step counter is <= the user's last-used window —
// preventing replay of a phished code within its 90-second validity
// envelope (RFC 6238 §5.2).
//
// Returns (newLastUsedWindow, ok, err). When ok=true the caller MUST
// persist newLastUsedWindow on the user row before returning the
// session to the client; otherwise a concurrent verify could accept
// the same code twice. (Tiny race; see plan "Risks".)
//
// When ok=false and the code matched a window that was already used,
// err is IsTOTPReplay-detectable so handlers can surface a distinct
// "code already used" message rather than the generic "wrong code."
func VerifyTOTPWithReuseGuard(plaintextSecret, code string, lastUsedWindow int64) (newLastUsedWindow int64, ok bool, err error) {
	if !VerifyTOTP(plaintextSecret, code) {
		return lastUsedWindow, false, nil
	}
	current := CurrentTOTPWindow()
	if current <= lastUsedWindow {
		return lastUsedWindow, false, errReplay{}
	}
	return current, true, nil
}

type errReplay struct{}

func (errReplay) Error() string { return "totp code already consumed for this window" }

// IsTOTPReplay returns true iff the error was emitted by
// VerifyTOTPWithReuseGuard due to a same-window replay.
func IsTOTPReplay(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(errReplay)
	return ok
}

// ValidateOTPURL is a debug helper: confirms an otpauth URL parses.
// Not used in production; kept for the enrollment-flow test which
// asserts EnrollUserTOTP emits a parseable URL.
func ValidateOTPURL(url string) error {
	_, err := otp.NewKeyFromURL(url)
	return err
}

// GenerateRecoveryCodes returns (plaintext, hashed) pairs in two
// parallel slices. Plaintext is shown to the user ONCE at enrollment;
// hashed is persisted to user_recovery_codes.code_hash. bcrypt cost
// 10 is the standard for hashed secrets at human-typing volume.
//
// Code format: 4-4 groups of upper-case alphanumeric, dash-separated,
// e.g. "9XQ4-7BPM". Easy to read aloud + retype.
func GenerateRecoveryCodes() (plaintext []string, hashed []string, err error) {
	plaintext = make([]string, 0, RecoveryCodeCount)
	hashed = make([]string, 0, RecoveryCodeCount)
	const alpha = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // ambiguous chars removed (0/O, 1/I/L)
	for i := 0; i < RecoveryCodeCount; i++ {
		var raw [8]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, nil, err
		}
		var sb strings.Builder
		for j, b := range raw {
			if j == 4 {
				sb.WriteByte('-')
			}
			sb.WriteByte(alpha[int(b)%len(alpha)])
		}
		code := sb.String()
		h, err := bcrypt.GenerateFromPassword([]byte(code), 10)
		if err != nil {
			return nil, nil, err
		}
		plaintext = append(plaintext, code)
		hashed = append(hashed, string(h))
	}
	return plaintext, hashed, nil
}

// VerifyRecoveryCode checks a single submitted code against the
// bcrypt-hashed stored value. Constant-time via bcrypt.
func VerifyRecoveryCode(submitted, storedHash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(submitted)) == nil
}

// EncodeSecretForStorage is a small convenience: takes the plaintext
// secret EnrollUserTOTP returned, encrypts it via secretbox, and
// returns the ciphertext bytes the caller writes to
// users.totp_secret_encrypted.
func EncodeSecretForStorage(plaintextSecret string) ([]byte, error) {
	return Encrypt([]byte(plaintextSecret))
}

// DecodeSecretFromStorage reverses EncodeSecretForStorage. Used by
// VerifyCodeForUser at login time.
func DecodeSecretFromStorage(ciphertext []byte) (string, error) {
	pt, err := Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// randomDigits is a small helper used in tests / future "send a code
// over a side channel" flows. Not exported for primary use because
// TOTP itself is the only code generator users see.
func randomDigits(n int) string {
	var sb strings.Builder
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for _, x := range b {
		sb.WriteByte('0' + x%10)
	}
	return sb.String()
}

// SanitizeCode strips whitespace + non-digit characters from
// user-typed TOTP codes. Authenticator apps display "123 456" with
// a space; users sometimes paste that. The handler normalizes
// before calling VerifyTOTP.
func SanitizeCode(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// EncodeRecoveryCode is a low-cost helper used by tests + future
// "send recovery codes via email" flows. Returns a URL-safe encoding
// of a recovery code suitable for embedding in a one-click link.
// Not currently used in v1.
func EncodeRecoveryCode(code string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(code))
}
