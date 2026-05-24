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

// TestGenerateToken_HasIssuerAndAudience asserts every session token
// carries the iss + aud claims required for middleware validation. A
// token minted for an unrelated service that happens to share
// JWT_SECRET must not be accepted as a Paper LMS session — these
// claims are the gate. Audit finding #5, 2026-05-22.
func TestGenerateToken_HasIssuerAndAudience(t *testing.T) {
	user := testUser()
	secret := "test-secret-key"

	tokenString, err := auth.GenerateToken(user, secret)
	assert.NoError(t, err)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	assert.NoError(t, err)

	claims, ok := token.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, auth.JWTIssuer, claims["iss"])

	// "aud" is encoded as []interface{} when there is exactly one
	// audience; both shapes (string and slice) are valid per RFC 7519.
	switch v := claims["aud"].(type) {
	case string:
		assert.Equal(t, auth.JWTAudienceAPI, v)
	case []interface{}:
		assert.Contains(t, v, auth.JWTAudienceAPI)
	default:
		t.Fatalf("aud claim has unexpected type %T", v)
	}
}

// TestGenerateMasqueradeToken_HasIssuerAndAudience mirrors the above
// for masquerade tokens — those follow the same validation path
// through the auth middleware.
func TestGenerateMasqueradeToken_HasIssuerAndAudience(t *testing.T) {
	target := testUser()
	secret := "test-secret-key"

	tokenString, err := auth.GenerateMasqueradeToken(target, 1, 1, secret)
	assert.NoError(t, err)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	assert.NoError(t, err)

	claims, ok := token.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, auth.JWTIssuer, claims["iss"])
}

// TestGenerateToken_RejectedByWrongAudience verifies the parser-side
// rejection: a token minted with a different aud cannot pass through
// a parser configured with the API audience expectation. This is the
// downgrade attack surface the iss/aud claims close.
func TestGenerateToken_RejectedByWrongAudience(t *testing.T) {
	secret := "test-secret-key"

	// Mint a token manually with a different audience.
	now := time.Now()
	mintedForOtherService := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  42,
		"iss": auth.JWTIssuer,
		"aud": "some-other-service",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	tokenString, err := mintedForOtherService.SignedString([]byte(secret))
	assert.NoError(t, err)

	// Parse with the API audience requirement — must fail.
	_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithIssuer(auth.JWTIssuer), jwt.WithAudience(auth.JWTAudienceAPI))
	assert.Error(t, err)
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
