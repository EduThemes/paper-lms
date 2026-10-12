package service_test

// Service-level regression tests for the integrations parity wave
// (B085, B181, B183, B184, B191, B192, B193).

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// --- B181: persisted, replica-shared LTI platform key -----------------------

type memPlatformKeyRepo struct {
	mu   sync.Mutex
	rows map[string]models.LTIPlatformKey
}

func (r *memPlatformKeyRepo) FindByName(_ context.Context, name string) (*models.LTIPlatformKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return &row, nil
}

func (r *memPlatformKeyRepo) CreateIfAbsent(_ context.Context, k *models.LTIPlatformKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rows == nil {
		r.rows = map[string]models.LTIPlatformKey{}
	}
	if _, ok := r.rows[k.Name]; !ok {
		r.rows[k.Name] = *k
	}
	return nil
}

// xorSeal is a stand-in for secretbox: reversible, and visibly not plaintext.
func xorSeal(b []byte) ([]byte, error) {
	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[i] ^ 0x5a
	}
	return out, nil
}

func newLTIServiceForKeyTest(t *testing.T) *service.LTIService {
	t.Helper()
	svc, err := service.NewLTIService(nil, nil, nil, nil, nil, nil, "https://lms.example.com")
	require.NoError(t, err)
	return svc
}

func jwksKid(t *testing.T, svc *service.LTIService) string {
	t.Helper()
	jwks, err := svc.GetJWKS()
	require.NoError(t, err)
	return jwks["keys"].([]interface{})[0].(map[string]interface{})["kid"].(string)
}

func TestLTIPlatformKey_SharedAcrossReplicasAndRestarts(t *testing.T) {
	repo := &memPlatformKeyRepo{}
	ctx := context.Background()

	podA := newLTIServiceForKeyTest(t)
	podB := newLTIServiceForKeyTest(t)
	require.NotEqual(t, jwksKid(t, podA), jwksKid(t, podB), "ephemeral keys differ — the bug")

	require.NoError(t, podA.LoadPersistentPlatformKey(ctx, repo, "", xorSeal, xorSeal))
	require.NoError(t, podB.LoadPersistentPlatformKey(ctx, repo, "", xorSeal, xorSeal))
	assert.Equal(t, jwksKid(t, podA), jwksKid(t, podB), "every replica must serve the same kid")

	restarted := newLTIServiceForKeyTest(t)
	require.NoError(t, restarted.LoadPersistentPlatformKey(ctx, repo, "", xorSeal, xorSeal))
	assert.Equal(t, jwksKid(t, podA), jwksKid(t, restarted), "the key must survive a restart")

	// The stored key is sealed, not a plaintext DER blob.
	row, err := repo.FindByName(ctx, "default")
	require.NoError(t, err)
	assert.NotEqual(t, byte(0x30), row.PrivateKeyEncrypted[0], "PKCS#8 DER must not be stored in the clear")
}

// --- B183: token dies with its developer key ---------------------------------

func TestValidateToken_RejectsTokenOfInactiveDeveloperKey(t *testing.T) {
	for _, state := range []string{"inactive", "deleted"} {
		repo := new(mocks.MockAccessTokenRepository)
		keyID := uint(5)
		future := time.Now().Add(time.Hour)
		repo.On("FindByToken", mock.Anything, sha256Hex("raw-token")).Return(&models.AccessToken{
			ID: 1, UserID: 10, DeveloperKeyID: &keyID, WorkflowState: "active", ExpiresAt: &future,
			DeveloperKey: &models.DeveloperKey{ID: 5, WorkflowState: state},
		}, nil)
		svc := service.NewAccessTokenService(repo)
		_, err := svc.ValidateToken(context.Background(), "raw-token")
		assert.Error(t, err, state)
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	}
}

// --- B184 / B193: refresh grant ---------------------------------------------------

func TestRefreshOAuthToken_BoundToClientAndNotRotated(t *testing.T) {
	repo := new(mocks.MockAccessTokenRepository)
	keyA := uint(5)
	refresh := sha256Hex("refresh-a")
	oldAccess := "old-access-hash"
	tok := &models.AccessToken{ID: 9, UserID: 10, DeveloperKeyID: &keyA, RefreshToken: &refresh, Token: oldAccess, WorkflowState: "active"}
	repo.On("FindByRefreshToken", mock.Anything, refresh).Return(tok, nil)
	repo.On("Update", mock.Anything, tok).Return(nil)
	svc := service.NewAccessTokenService(repo)

	_, _, err := svc.RefreshOAuthToken(context.Background(), "refresh-a", 6)
	assert.Error(t, err, "another client's refresh token must be refused")

	updated, rawAccess, err := svc.RefreshOAuthToken(context.Background(), "refresh-a", 5)
	require.NoError(t, err)
	assert.Equal(t, sha256Hex(rawAccess), updated.Token)
	assert.NotEqual(t, oldAccess, updated.Token, "previous access token is replaced")
	assert.Equal(t, &refresh, updated.RefreshToken, "refresh token is kept (Canvas parity)")
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)

	// The same refresh token keeps working for a second refresh.
	_, _, err = svc.RefreshOAuthToken(context.Background(), "refresh-a", 5)
	assert.NoError(t, err)
}

// --- B085: scope matching ----------------------------------------------------------

func TestScopeAllowsRequest(t *testing.T) {
	scopes := []string{
		"url:GET|/api/v1/courses/:course_id/assignments",
		"url:POST|/api/v1/courses/:course_id/assignments/:assignment_id/submissions(.:format)",
		"url:GET|/api/v1/files/*path",
	}
	cases := []struct {
		method, path string
		want         bool
	}{
		{"GET", "/api/v1/courses/12/assignments", true},
		{"HEAD", "/api/v1/courses/12/assignments", true},
		{"DELETE", "/api/v1/courses/12/assignments", false},
		{"GET", "/api/v1/courses/12/assignments/9", false},
		{"GET", "/api/v1/courses/12/quizzes", false},
		{"POST", "/api/v1/courses/12/assignments/9/submissions", true},
		{"GET", "/api/v1/files/a/b/c", true},
		{"PUT", "/api/v1/courses/12", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, service.ScopeAllowsRequest(scopes, tc.method, tc.path), "%s %s", tc.method, tc.path)
	}
}

func TestTokenScopeRestriction(t *testing.T) {
	keyID := uint(5)
	enforced := &models.AccessToken{DeveloperKeyID: &keyID, Scopes: `["url:GET|/api/v1/courses"]`,
		DeveloperKey: &models.DeveloperKey{RequireScopes: true}}
	scopes, on := service.TokenScopeRestriction(enforced)
	assert.True(t, on)
	assert.Equal(t, []string{"url:GET|/api/v1/courses"}, scopes)

	lax := &models.AccessToken{DeveloperKeyID: &keyID, Scopes: `["url:GET|/api/v1/courses"]`,
		DeveloperKey: &models.DeveloperKey{RequireScopes: false}}
	_, on = service.TokenScopeRestriction(lax)
	assert.False(t, on, "Canvas only enforces scopes for require_scopes keys")

	pat := &models.AccessToken{Scopes: `["url:GET|/api/v1/courses"]`}
	_, on = service.TokenScopeRestriction(pat)
	assert.True(t, on, "a PAT created with scopes is restricted to them")

	_, on = service.TokenScopeRestriction(&models.AccessToken{Scopes: "[]"})
	assert.False(t, on)
}

// --- B191: client secret verification -------------------------------------------

func TestVerifyClientSecret_HashedAndLegacy(t *testing.T) {
	hashed := &models.DeveloperKey{ClientSecret: service.HashClientSecret("s3cret")}
	assert.True(t, service.VerifyClientSecret(hashed, "s3cret"))
	assert.False(t, service.VerifyClientSecret(hashed, "wrong"))
	assert.False(t, service.VerifyClientSecret(hashed, hashed.ClientSecret), "the stored hash is not a credential")

	legacy := &models.DeveloperKey{ClientSecret: "plain-legacy"}
	assert.True(t, service.VerifyClientSecret(legacy, "plain-legacy"))
	assert.False(t, service.VerifyClientSecret(legacy, ""))
}

// --- B192: authorization codes are shared via the code repository ---------------

type memCodeRepo struct {
	mu   sync.Mutex
	rows map[string]models.OAuthAuthorizationCode
}

func (r *memCodeRepo) Create(_ context.Context, c *models.OAuthAuthorizationCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rows == nil {
		r.rows = map[string]models.OAuthAuthorizationCode{}
	}
	r.rows[c.CodeHash] = *c
	return nil
}

func (r *memCodeRepo) Consume(_ context.Context, h string) (*models.OAuthAuthorizationCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[h]
	if !ok {
		return nil, errors.New("not found")
	}
	delete(r.rows, h)
	return &row, nil
}

func (r *memCodeRepo) DeleteExpired(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range r.rows {
		if time.Now().After(v.ExpiresAt) {
			delete(r.rows, k)
		}
	}
	return nil
}

func TestOAuth2Codes_RedeemableOnAnotherReplicaAndSingleUse(t *testing.T) {
	devRepo := new(mocks.MockDeveloperKeyRepository)
	devRepo.On("FindByClientID", mock.Anything, "dk_example").Return(&models.DeveloperKey{
		ID: 5, AccountID: 1, ClientID: "dk_example", ClientSecret: service.HashClientSecret("s3cret"),
		RedirectURI: "https://app.example.com/cb", WorkflowState: "active",
	}, nil)
	tokenRepo := new(mocks.MockAccessTokenRepository)
	tokenRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	shared := &memCodeRepo{}
	// An expired, abandoned code is purged by the next authorization.
	_ = shared.Create(context.Background(), &models.OAuthAuthorizationCode{CodeHash: "stale", ExpiresAt: time.Now().Add(-time.Minute)})

	newReplica := func() *service.OAuth2Service {
		s := service.NewOAuth2Service(service.NewDeveloperKeyService(devRepo), service.NewAccessTokenService(tokenRepo))
		s.SetCodeRepository(shared)
		return s
	}
	podA, podB := newReplica(), newReplica()

	code, err := podA.GenerateAuthorizationCode(context.Background(), 10, 1, "dk_example", "https://app.example.com/cb", nil)
	require.NoError(t, err)
	_, staleLeft := shared.rows["stale"]
	assert.False(t, staleLeft, "expired codes are purged")

	_, access, refresh, err := podB.ExchangeCode(context.Background(), code, "dk_example", "s3cret", "https://app.example.com/cb")
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)

	_, _, _, err = podA.ExchangeCode(context.Background(), code, "dk_example", "s3cret", "https://app.example.com/cb")
	assert.Error(t, err, "codes are single-use across replicas")

	// B076: a user of another tenant cannot authorize tenant 1's client.
	_, err = podA.GenerateAuthorizationCode(context.Background(), 11, 2, "dk_example", "https://app.example.com/cb", nil)
	assert.Error(t, err)
}
