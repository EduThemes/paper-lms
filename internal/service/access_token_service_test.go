package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// sha256Hex computes the SHA-256 hex digest of a string, matching the
// hashToken implementation in access_token_service.go.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// CreatePersonalAccessToken
// ---------------------------------------------------------------------------

func TestCreatePersonalAccessToken_Success(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	token, rawToken, err := svc.CreatePersonalAccessToken(ctx, 1, "CI deploys", []string{"read", "write"})

	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.NotEmpty(t, rawToken)
	assert.Len(t, rawToken, 64, "raw token should be a 64-char hex string")
	assert.Equal(t, uint(1), token.UserID)
	assert.Equal(t, "CI deploys", token.Purpose)
	assert.Equal(t, "active", token.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestCreatePersonalAccessToken_Hint(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	token, rawToken, err := svc.CreatePersonalAccessToken(ctx, 1, "testing", nil)

	assert.NoError(t, err)
	assert.NotNil(t, token)
	// The hint should be the last 4 characters of the raw token
	expectedHint := rawToken[len(rawToken)-4:]
	assert.Equal(t, expectedHint, token.TokenHint)
	mockRepo.AssertExpectations(t)
}

func TestCreatePersonalAccessToken_Hashed(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	token, rawToken, err := svc.CreatePersonalAccessToken(ctx, 1, "testing", nil)

	assert.NoError(t, err)
	assert.NotNil(t, token)
	// The stored Token must NOT equal the raw token — it should be the SHA-256 hash
	assert.NotEqual(t, rawToken, token.Token, "stored token must be hashed, not raw")
	assert.Equal(t, sha256Hex(rawToken), token.Token, "stored token should be the SHA-256 of the raw token")
	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// CreateOAuthToken
// ---------------------------------------------------------------------------

func TestCreateOAuthToken_Success(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	token, rawAccess, rawRefresh, err := svc.CreateOAuthToken(ctx, 1, 10, []string{"openid"})

	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.NotEmpty(t, rawAccess)
	assert.NotEmpty(t, rawRefresh)
	assert.Len(t, rawAccess, 64)
	assert.Len(t, rawRefresh, 64)
	assert.NotEqual(t, rawAccess, rawRefresh, "access and refresh tokens must differ")
	assert.Equal(t, uint(1), token.UserID)
	assert.NotNil(t, token.DeveloperKeyID)
	assert.Equal(t, uint(10), *token.DeveloperKeyID)
	assert.NotNil(t, token.RefreshToken)
	assert.Equal(t, "active", token.WorkflowState)
	mockRepo.AssertExpectations(t)
}

func TestCreateOAuthToken_Expires(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	mockRepo.On("Create", ctx, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	before := time.Now()
	token, _, _, err := svc.CreateOAuthToken(ctx, 1, 10, nil)
	after := time.Now()

	assert.NoError(t, err)
	assert.NotNil(t, token.ExpiresAt, "OAuth token must have an ExpiresAt")

	// ExpiresAt should be approximately 1 hour from now
	expectedEarliest := before.Add(1*time.Hour - 2*time.Second)
	expectedLatest := after.Add(1*time.Hour + 2*time.Second)
	assert.True(t, token.ExpiresAt.After(expectedEarliest),
		"ExpiresAt %v should be after %v", token.ExpiresAt, expectedEarliest)
	assert.True(t, token.ExpiresAt.Before(expectedLatest),
		"ExpiresAt %v should be before %v", token.ExpiresAt, expectedLatest)
	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ValidateToken
// ---------------------------------------------------------------------------

func TestValidateToken_Success(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	rawToken := "abc123def456abc123def456abc123def456abc123def456abc123def456abcd"
	tokenHash := sha256Hex(rawToken)

	storedToken := &models.AccessToken{
		ID:            1,
		UserID:        42,
		Token:         tokenHash,
		WorkflowState: "active",
	}

	mockRepo.On("FindByToken", ctx, tokenHash).Return(storedToken, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	result, err := svc.ValidateToken(ctx, rawToken)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint(42), result.UserID)
	assert.NotNil(t, result.LastUsedAt, "LastUsedAt should be updated")
	mockRepo.AssertExpectations(t)
}

func TestValidateToken_Invalid(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	rawToken := "nonexistenttoken"
	tokenHash := sha256Hex(rawToken)

	mockRepo.On("FindByToken", ctx, tokenHash).Return(nil, errors.New("not found"))

	result, err := svc.ValidateToken(ctx, rawToken)

	assert.Nil(t, result)
	assert.EqualError(t, err, "invalid token")
	mockRepo.AssertExpectations(t)
}

func TestValidateToken_Revoked(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	rawToken := "revokedtoken1234revokedtoken1234revokedtoken1234revokedtoken1234"
	tokenHash := sha256Hex(rawToken)

	storedToken := &models.AccessToken{
		ID:            2,
		UserID:        42,
		Token:         tokenHash,
		WorkflowState: "deleted",
	}

	mockRepo.On("FindByToken", ctx, tokenHash).Return(storedToken, nil)

	result, err := svc.ValidateToken(ctx, rawToken)

	assert.Nil(t, result)
	assert.EqualError(t, err, "token has been revoked")
	mockRepo.AssertExpectations(t)
}

func TestValidateToken_Expired(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	rawToken := "expiredtoken1234expiredtoken1234expiredtoken1234expiredtoken1234"
	tokenHash := sha256Hex(rawToken)

	pastTime := time.Now().Add(-2 * time.Hour)
	storedToken := &models.AccessToken{
		ID:            3,
		UserID:        42,
		Token:         tokenHash,
		WorkflowState: "active",
		ExpiresAt:     &pastTime,
	}

	mockRepo.On("FindByToken", ctx, tokenHash).Return(storedToken, nil)

	result, err := svc.ValidateToken(ctx, rawToken)

	assert.Nil(t, result)
	assert.EqualError(t, err, "token has expired")
	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// RevokeToken
// ---------------------------------------------------------------------------

func TestRevokeToken_Success(t *testing.T) {
	mockRepo := new(mocks.MockAccessTokenRepository)
	svc := service.NewAccessTokenService(mockRepo)
	ctx := context.Background()

	storedToken := &models.AccessToken{
		ID:            5,
		UserID:        42,
		WorkflowState: "active",
	}

	mockRepo.On("FindByID", ctx, uint(5)).Return(storedToken, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*models.AccessToken")).
		Run(func(args mock.Arguments) {
			tok := args.Get(1).(*models.AccessToken)
			assert.Equal(t, "deleted", tok.WorkflowState,
				"RevokeToken should set WorkflowState to deleted")
		}).
		Return(nil)

	err := svc.RevokeToken(ctx, 5, 42)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
