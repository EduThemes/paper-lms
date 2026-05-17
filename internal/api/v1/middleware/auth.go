package middleware

import (
	"strings"

	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	jwtSecret          string
	accessTokenService *service.AccessTokenService
	userRepo           repository.UserRepository
	tokenBlacklist     *service.TokenBlacklist
}

func NewAuthMiddleware(jwtSecret string, accessTokenService *service.AccessTokenService, userRepo repository.UserRepository, tokenBlacklist *service.TokenBlacklist) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:          jwtSecret,
		accessTokenService: accessTokenService,
		userRepo:           userRepo,
		tokenBlacklist:     tokenBlacklist,
	}
}

func (m *AuthMiddleware) Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var tokenStr string

		// 1. Check Authorization header first (API tokens, OAuth2, programmatic access)
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			tokenParts := strings.SplitN(authHeader, " ", 2)
			if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
				tokenStr = tokenParts[1]
			}
		}

		// 2. Fall back to httpOnly session cookie (browser-based access)
		if tokenStr == "" {
			tokenStr = c.Cookies("paper_session")
		}

		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": "Unauthorized - no token provided"}},
			})
		}

		// Try JWT first (session tokens from login)
		jwtToken, jwtErr := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(m.jwtSecret), nil
		})

		if jwtErr == nil && jwtToken.Valid {
			// Check if token was revoked via logout
			if m.tokenBlacklist != nil && m.tokenBlacklist.IsRevoked(tokenStr) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"errors": []fiber.Map{{"message": "Token has been revoked"}},
				})
			}
			claims, ok := jwtToken.Claims.(jwt.MapClaims)
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"errors": []fiber.Map{{"message": "Invalid token claims"}},
				})
			}
			idFloat, ok := claims["id"].(float64)
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"errors": []fiber.Map{{"message": "Invalid token: missing user ID"}},
				})
			}
			email, _ := claims["email"].(string)
			c.Locals("user_id", uint(idFloat))
			c.Locals("user_email", email)
			if name, ok := claims["name"].(string); ok {
				c.Locals("user_name", name)
			}
			// Populate is_admin Locals at auth time so EVERY handler
			// can read it without needing the permissions middleware
			// chain. Closes F4.3 from
			// docs/audits/2026-05-15-gamification-audit.md. Tokens
			// minted before this claim was added simply default to
			// false — permissions.RequireAdmin / RequireSelfOrAdmin
			// still re-validate via the userRepo at every protected
			// admin route, so a missing claim is safe (no escalation
			// path), just degrades to the previous "DB lookup at
			// permission middleware" behavior.
			// JWT-claim path: populate user_role for UI personalization
			// only. is_admin is derived from the claim as a soft hint
			// — handlers that act on the value go through RequireAdmin
			// (which re-fetches the user row and authoritatively sets
			// is_admin after the DB check). The is_super_admin flag is
			// NOT derived from the JWT claim: a demoted super_admin
			// still carries the claim in their unexpired JWT, and any
			// gate that trusted the claim would honor the stale role
			// for up to the token's TTL. Only PermissionMiddleware's
			// RequireAdmin (super_admin branch), RequireSuperAdmin, and
			// the isAdmin helper — all DB re-checking — are authorized
			// to set is_super_admin Locals. See the 2026-05-17 Wave 2
			// audit, finding M3.
			if role, ok := claims["role"].(string); ok {
				c.Locals("user_role", role)
				c.Locals("is_admin", role == "admin" || role == "super_admin")
			}
			// 13.1.B — tenant scope. New tokens carry account_id; old
			// tokens don't. For an old token we look up the user once
			// and 401 if their DB row also lacks an account_id (can't
			// happen post-13.1.A migration, but the guard makes the
			// invariant explicit).
			if acctFloat, ok := claims["account_id"].(float64); ok && acctFloat > 0 {
				c.Locals("account_id", uint(acctFloat))
			} else if m.userRepo != nil {
				if user, lookupErr := m.userRepo.FindByID(c.Context(), uint(idFloat)); lookupErr == nil && user != nil {
					if user.AccountID == 0 {
						return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
							"errors": []fiber.Map{{"message": "session predates tenant assignment; please log in again"}},
						})
					}
					c.Locals("account_id", user.AccountID)
				}
			}
			// admin_account_id captures a masquerade's REAL home tenant
			// so audit emitters can attribute back to the impersonator.
			if adminAcct, ok := claims["admin_account_id"].(float64); ok && adminAcct > 0 {
				c.Locals("admin_account_id", uint(adminAcct))
			}
			// If this is a masquerade token, set the masquerade_by local
			// so handlers can detect masquerade sessions
			if masqueradeBy, ok := claims["masquerade_by"].(float64); ok {
				c.Locals("masquerade_by", uint(masqueradeBy))
			}
			return c.Next()
		}

		// Try access token (Personal Access Tokens and OAuth2 tokens)
		if m.accessTokenService != nil {
			accessToken, err := m.accessTokenService.ValidateToken(c.Context(), tokenStr)
			if err == nil {
				c.Locals("user_id", accessToken.UserID)

				// Look up user for email, name, and role. The role
				// drives is_admin Locals (closes F4.3); since the
				// access-token path always does a user lookup anyway,
				// adding the role is free.
				if m.userRepo != nil {
					user, userErr := m.userRepo.FindByID(c.Context(), accessToken.UserID)
					if userErr == nil {
						c.Locals("user_email", user.Email)
						c.Locals("user_name", user.Name)
						// Access-token path: same provenance contract as
						// the JWT path. user_role + is_admin set as a
						// soft hint; is_super_admin Locals is the
						// exclusive output of RequireSuperAdmin / the
						// isAdmin helper after their DB re-check. See
						// 2026-05-17 Wave 2 audit finding M3.
						c.Locals("user_role", user.Role)
						c.Locals("is_admin", user.Role == "admin" || user.Role == "super_admin")
						// 13.1.B — tenant for access-token sessions.
						if user.AccountID > 0 {
							c.Locals("account_id", user.AccountID)
						}
					}
				}

				c.Locals("access_token_id", accessToken.ID)
				c.Locals("token_scopes", accessToken.Scopes)
				return c.Next()
			}
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"errors": []fiber.Map{{"message": "Invalid or expired token"}},
		})
	}
}
