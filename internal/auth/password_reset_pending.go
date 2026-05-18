package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Password-reset pending tokens are a SEPARATE JWT type from regular
// session tokens AND from MFA pending tokens. The `purpose` claim
// (encoded in the JWT ID) distinguishes them; any handler that reads
// a session token MUST reject `purpose="password_reset_pending"` and
// the password-set endpoint MUST verify the purpose matches. Same
// "fail closed" pattern as MFA pending — a forgotten check on either
// side reveals itself immediately rather than silently granting
// elevated access.
//
// TTL is 10 minutes. A pending token is single-purpose and single-
// step: it carries the user across the credential-verify → password-
// set boundary. If the user takes more than 10 minutes to choose a
// new password, they log in again (with the random SIS-imported
// password they don't know, so functionally they'd contact an admin
// — same recovery surface as before).
//
// Storage: never in localStorage. Frontend uses sessionStorage so
// the token clears on tab close.

const (
	purposePasswordResetPending = "password_reset_pending"
	passwordResetPendingTTL     = 10 * time.Minute
)

// PendingPasswordResetClaims are the JWT body for a pending-
// password-set token. AccountID is carried so the eventual session
// JWT minted on success preserves tenant scope without a re-fetch.
type PendingPasswordResetClaims struct {
	UserID    uint `json:"user_id"`
	AccountID uint `json:"account_id"`
	jwt.RegisteredClaims
}

// IssuePendingPasswordResetToken mints a signed pending-password-set
// JWT. The pipeline returns this in the login response instead of a
// real session token when the user has RequiresPasswordReset=true.
func IssuePendingPasswordResetToken(jwtSecret string, userID, accountID uint) (string, error) {
	claims := PendingPasswordResetClaims{
		UserID:    userID,
		AccountID: accountID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(passwordResetPendingTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "purpose:" + purposePasswordResetPending,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// VerifyPendingPasswordResetToken parses + validates a pending-
// password-set JWT, returning (user_id, account_id) on success.
// Rejects any token whose JWT ID claim doesn't carry the pending-
// password-reset purpose marker — a regular session token can't be
// reused as a pending token, and vice versa. An MFA pending token
// also fails this check because its purpose marker is different.
func VerifyPendingPasswordResetToken(jwtSecret, tokenStr string) (userID, accountID uint, err error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &PendingPasswordResetClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return 0, 0, err
	}
	claims, ok := parsed.Claims.(*PendingPasswordResetClaims)
	if !ok || !parsed.Valid {
		return 0, 0, errors.New("invalid pending-password-reset token")
	}
	if claims.ID != "purpose:"+purposePasswordResetPending {
		return 0, 0, errors.New("token is not a pending-password-reset token")
	}
	return claims.UserID, claims.AccountID, nil
}
