package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// JWT issuer and audience values asserted on every Paper LMS session
// token. Validating both prevents a token minted for another service
// (or a different Paper LMS subsystem) from being accepted as a
// session token if JWT_SECRET is ever shared across services.
// Documented as audit finding #5, 2026-05-22.
const (
	JWTIssuer       = "paper-lms"
	JWTAudienceAPI  = "paper-lms-api"
)

func GenerateToken(user *models.User, secret string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
		// "role" lets the auth middleware populate is_admin Locals
		// without a per-request DB lookup. Closes F4.3 from
		// docs/audits/2026-05-15-gamification-audit.md (handlers that
		// read is_admin without the permission middleware mounted
		// previously got `false` for admins).
		"role": user.Role,
		// "account_id" is the tenant scope, added in Phase 13 / 13.1.B.
		// Auth middleware sets c.Locals("account_id"); every tenant-
		// scoped repo read filters by this claim. Pre-13.1.B tokens
		// don't carry the claim and fall back to a userRepo lookup +
		// 401 if the DB column is also null.
		"account_id": user.AccountID,
		"iss":        JWTIssuer,
		"aud":        JWTAudienceAPI,
		"iat":        now.Unix(),
		"exp":        now.Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(secret))
}

// GenerateMasqueradeToken creates a JWT token for the target user with an extra
// masquerade_by claim containing the admin's user ID. This allows the auth
// middleware to identify masquerade sessions.
//
// Note: the `role` claim on a masquerade token is the TARGET user's role,
// not the admin's. is_admin Locals therefore reflects the masquerade
// target's permissions, which is the correct mental model — the admin
// is impersonating, not exercising admin powers.
//
// Tenant: `account_id` reflects the TARGET user's tenant (so any tenant-
// scoped read sees the masquerade's surface, not the admin's).
// `admin_account_id` carries the masquerader's home tenant so audit
// logging can attribute the action back to the real admin.
func GenerateMasqueradeToken(targetUser *models.User, adminUserID, adminAccountID uint, secret string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":               targetUser.ID,
		"email":            targetUser.Email,
		"name":             targetUser.Name,
		"role":             targetUser.Role,
		"masquerade_by":    adminUserID,
		"account_id":       targetUser.AccountID,
		"admin_account_id": adminAccountID,
		"iss":              JWTIssuer,
		"aud":              JWTAudienceAPI,
		"iat":              now.Unix(),
		"exp":              now.Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(secret))
}
