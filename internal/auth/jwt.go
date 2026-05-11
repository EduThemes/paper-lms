package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func GenerateToken(user *models.User, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(secret))
}

// GenerateMasqueradeToken creates a JWT token for the target user with an extra
// masquerade_by claim containing the admin's user ID. This allows the auth
// middleware to identify masquerade sessions.
func GenerateMasqueradeToken(targetUser *models.User, adminUserID uint, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":             targetUser.ID,
		"email":          targetUser.Email,
		"name":           targetUser.Name,
		"masquerade_by":  adminUserID,
		"exp":            time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(secret))
}
