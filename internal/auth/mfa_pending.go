package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MFA pending tokens are a separate JWT type from regular session
// tokens. The `purpose` claim distinguishes them; any handler that
// reads a session token MUST reject `purpose="mfa_pending"` and any
// handler that accepts a pending token MUST verify the purpose
// matches. This is the "fail closed" principle — a forgotten check on
// either side reveals itself immediately rather than silently
// granting elevated access.
//
// TTL is intentionally short (5 minutes). A pending token is single-
// purpose and single-step: it carries the user across the
// password/SSO → 2FA boundary. If the user takes more than 5 minutes
// to fish their phone out, they can log in again.
//
// Storage: never in localStorage. Frontend uses sessionStorage so the
// token clears on tab close, and a forgotten-on-shared-device pending
// token can't be replayed indefinitely.

const (
	purposeMFAPending = "mfa_pending"
	pendingTTL        = 5 * time.Minute
)

// PendingMFAClaims are the JWT body for a pending-2FA token.
type PendingMFAClaims struct {
	UserID         uint   `json:"user_id"`
	LoginProvider  string `json:"login_provider"` // "local" | "saml" | etc.
	jwt.RegisteredClaims
}

// IssuePendingMFAToken mints a signed pending-2FA JWT. The pipeline
// returns this in the login response instead of a real session token
// when the user is enrolled and the policy requires step-up.
func IssuePendingMFAToken(jwtSecret string, userID uint, loginProvider string) (string, error) {
	claims := PendingMFAClaims{
		UserID:        userID,
		LoginProvider: loginProvider,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(pendingTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "purpose:" + purposeMFAPending,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// VerifyPendingMFAToken parses + validates a pending-2FA JWT, returning
// the user_id and original login provider on success. Rejects any
// token whose JWT ID claim doesn't carry the pending-MFA purpose
// marker — a regular session token can't be reused as a pending
// token, and vice versa.
func VerifyPendingMFAToken(jwtSecret, tokenStr string) (userID uint, loginProvider string, err error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &PendingMFAClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return 0, "", err
	}
	claims, ok := parsed.Claims.(*PendingMFAClaims)
	if !ok || !parsed.Valid {
		return 0, "", errors.New("invalid pending-mfa token")
	}
	if claims.ID != "purpose:"+purposeMFAPending {
		return 0, "", errors.New("token is not a pending-mfa token")
	}
	return claims.UserID, claims.LoginProvider, nil
}
