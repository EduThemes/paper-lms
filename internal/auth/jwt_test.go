package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/assert"
)

func testUser() *models.User {
	return &models.User{
		ID:    42,
		Email: "alice@example.com",
		Name:  "Alice Wonderland",
	}
}

func TestGenerateToken_ValidClaims(t *testing.T) {
	user := testUser()
	secret := "test-secret-key"

	tokenString, err := auth.GenerateToken(user, secret)
	assert.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	// Parse the token back
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	assert.NoError(t, err)
	assert.True(t, token.Valid)

	claims, ok := token.Claims.(jwt.MapClaims)
	assert.True(t, ok)

	// Verify claims match user fields
	assert.Equal(t, float64(42), claims["id"])
	assert.Equal(t, "alice@example.com", claims["email"])
	assert.Equal(t, "Alice Wonderland", claims["name"])
}

func TestGenerateToken_Expiration(t *testing.T) {
	user := testUser()
	secret := "test-secret-key"

	before := time.Now()
	tokenString, err := auth.GenerateToken(user, secret)
	after := time.Now()
	assert.NoError(t, err)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	assert.NoError(t, err)

	claims, ok := token.Claims.(jwt.MapClaims)
	assert.True(t, ok)

	// exp should be ~24 hours from now
	expFloat, ok := claims["exp"].(float64)
	assert.True(t, ok)
	expTime := time.Unix(int64(expFloat), 0)

	expectedEarliest := before.Add(24 * time.Hour).Add(-1 * time.Second)
	expectedLatest := after.Add(24 * time.Hour).Add(1 * time.Second)

	assert.True(t, expTime.After(expectedEarliest), "exp %v should be after %v", expTime, expectedEarliest)
	assert.True(t, expTime.Before(expectedLatest), "exp %v should be before %v", expTime, expectedLatest)
}

func TestGenerateToken_SigningMethod(t *testing.T) {
	user := testUser()
	secret := "test-secret-key"

	tokenString, err := auth.GenerateToken(user, secret)
	assert.NoError(t, err)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is HS256
		assert.Equal(t, jwt.SigningMethodHS256, token.Method)
		return []byte(secret), nil
	})
	assert.NoError(t, err)
	assert.True(t, token.Valid)
	assert.Equal(t, "HS256", token.Method.Alg())
}

func TestGenerateToken_WrongSecret(t *testing.T) {
	user := testUser()
	correctSecret := "correct-secret"
	wrongSecret := "wrong-secret"

	tokenString, err := auth.GenerateToken(user, correctSecret)
	assert.NoError(t, err)

	// Parsing with the wrong secret should fail
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(wrongSecret), nil
	})
	assert.Error(t, err)
	assert.False(t, token.Valid)
}

func TestGenerateToken_HasSignature(t *testing.T) {
	user := testUser()
	secret := "test-secret-key"

	tokenString, err := auth.GenerateToken(user, secret)
	assert.NoError(t, err)

	// JWT should have exactly 3 parts: header.payload.signature
	parts := strings.Split(tokenString, ".")
	assert.Len(t, parts, 3, "JWT should have 3 parts (header.payload.signature)")

	// Each part should be non-empty
	for i, part := range parts {
		assert.NotEmpty(t, part, "JWT part %d should not be empty", i)
	}
}
