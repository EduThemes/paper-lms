package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Round-trip: issue → verify reads back the right user_id + account_id.
func TestPendingPasswordResetToken_RoundTrip(t *testing.T) {
	const secret = "test-jwt-secret"
	tok, err := IssuePendingPasswordResetToken(secret, 42, 7)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	uid, acct, err := VerifyPendingPasswordResetToken(secret, tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if uid != 42 {
		t.Errorf("user_id: got %d want 42", uid)
	}
	if acct != 7 {
		t.Errorf("account_id: got %d want 7", acct)
	}
}

// A regular session JWT must NOT pass the pending-password-reset
// verifier. The verifier rejects on the purpose marker, not on
// signature alone.
func TestPendingPasswordResetToken_RejectsSessionToken(t *testing.T) {
	const secret = "test-jwt-secret"
	// Mint a generic session-shaped token signed with the same secret.
	sessionTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  42,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := sessionTok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign session: %v", err)
	}
	if _, _, err := VerifyPendingPasswordResetToken(secret, signed); err == nil {
		t.Error("expected pending-password-reset verifier to reject a session token")
	}
}

// An MFA pending token must NOT pass the password-reset verifier and
// vice versa — the purpose markers are distinct.
func TestPendingPasswordResetToken_RejectsMFAPendingToken(t *testing.T) {
	const secret = "test-jwt-secret"
	mfaTok, err := IssuePendingMFAToken(secret, 42, "local")
	if err != nil {
		t.Fatalf("issue mfa pending: %v", err)
	}
	if _, _, err := VerifyPendingPasswordResetToken(secret, mfaTok); err == nil {
		t.Error("password-reset verifier must reject an MFA-pending token")
	}
	pwTok, err := IssuePendingPasswordResetToken(secret, 42, 7)
	if err != nil {
		t.Fatalf("issue pw pending: %v", err)
	}
	if _, _, err := VerifyPendingMFAToken(secret, pwTok); err == nil {
		t.Error("MFA-pending verifier must reject a password-reset token")
	}
}

// Wrong secret → verifier returns an error.
func TestPendingPasswordResetToken_WrongSecret(t *testing.T) {
	tok, err := IssuePendingPasswordResetToken("correct-secret", 42, 7)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, _, err := VerifyPendingPasswordResetToken("wrong-secret", tok); err == nil {
		t.Error("expected error verifying with wrong secret")
	}
}

// Garbage token → verifier returns an error.
func TestPendingPasswordResetToken_Garbage(t *testing.T) {
	if _, _, err := VerifyPendingPasswordResetToken("any", "not-a-jwt"); err == nil {
		t.Error("expected error parsing garbage token")
	}
}

// TTL: an issued token with a past exp must fail verification. We
// can't easily fast-forward time so we hand-mint a token with an
// expired exp.
func TestPendingPasswordResetToken_TTLExpired(t *testing.T) {
	const secret = "test-jwt-secret"
	claims := PendingPasswordResetClaims{
		UserID:    42,
		AccountID: 7,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-11 * time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-11 * time.Minute)),
			ID:        "purpose:" + purposePasswordResetPending,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign expired: %v", err)
	}
	if _, _, err := VerifyPendingPasswordResetToken(secret, signed); err == nil {
		t.Error("expected expired token to fail verification")
	} else if !strings.Contains(err.Error(), "expired") && !strings.Contains(err.Error(), "exp") {
		// jwt/v5 wraps validation errors; the substring check is
		// loose so library version bumps don't break the test.
		t.Logf("expired-token error message: %v", err)
	}
}

// TTL upper bound: an issued token must have an exp roughly
// passwordResetPendingTTL into the future (give or take a few
// seconds for test wall-clock jitter).
func TestPendingPasswordResetToken_TTLWindow(t *testing.T) {
	const secret = "test-jwt-secret"
	before := time.Now()
	tok, err := IssuePendingPasswordResetToken(secret, 42, 7)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	parsed, err := jwt.ParseWithClaims(tok, &PendingPasswordResetClaims{}, func(_ *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c := parsed.Claims.(*PendingPasswordResetClaims)
	delta := c.ExpiresAt.Time.Sub(before)
	if delta < passwordResetPendingTTL-5*time.Second || delta > passwordResetPendingTTL+5*time.Second {
		t.Errorf("exp delta %v not within ±5s of %v", delta, passwordResetPendingTTL)
	}
}
