package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
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

				// Look up user for email and name
				if m.userRepo != nil {
					user, userErr := m.userRepo.FindByID(c.Context(), accessToken.UserID)
					if userErr == nil {
						c.Locals("user_email", user.Email)
						c.Locals("user_name", user.Name)
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
