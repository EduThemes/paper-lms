package handlers_test

// Regression tests for the integrations parity wave: developer keys,
// OAuth2 authorization/token endpoints, personal access tokens and
// external tools (B075, B076, B084, B182, B184, B185, B190, B191, B193).

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func callerStub(userID, accountID uint, enrollmentType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		if enrollmentType != "" {
			c.Locals("enrollment_type", enrollmentType)
		}
		return c.Next()
	}
}

// --- B075 / B182: developer keys are tenant-owned -------------------------

func devKeysApp(repo *mocks.MockDeveloperKeyRepository, callerAccount uint) *fiber.App {
	h := handlers.NewDeveloperKeyHandler(service.NewDeveloperKeyService(repo))
	app := testutil.SetupTestApp()
	app.Use(callerStub(10, callerAccount, ""), middleware.PaginationParams())
	app.Get("/accounts/:account_id/developer_keys", h.ListDeveloperKeys)
	app.Post("/accounts/:account_id/developer_keys", h.CreateDeveloperKey)
	return app
}

func TestDeveloperKeys_ListAndCreate_RefuseOtherTenantAccountID(t *testing.T) {
	repo := new(mocks.MockDeveloperKeyRepository)
	app := devKeysApp(repo, 1)

	resp := testutil.MakeRequest(app, http.MethodGet, "/accounts/2/developer_keys", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = testutil.MakeRequest(app, http.MethodPost, "/accounts/2/developer_keys",
		testutil.JSONBody(map[string]interface{}{"developer_key": map[string]interface{}{"name": "Evil"}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	repo.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestDeveloperKeys_SelfAliasListsCallerTenant(t *testing.T) {
	repo := new(mocks.MockDeveloperKeyRepository)
	repo.On("List", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.DeveloperKey]{
		Items: []models.DeveloperKey{{ID: 3, AccountID: 1, Name: "Mine", Scopes: `["url:GET|/api/v1/courses"]`}},
		TotalCount: 1, Page: 1, PerPage: 10,
	}, nil)
	app := devKeysApp(repo, 1)

	for _, path := range []string{"/accounts/self/developer_keys", "/accounts/1/developer_keys"} {
		resp := testutil.MakeRequest(app, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		arr, err := testutil.ParseJSONArray(resp)
		require.NoError(t, err)
		require.Len(t, arr, 1)
		assert.Equal(t, []interface{}{"url:GET|/api/v1/courses"}, arr[0]["scopes"])
	}
}

func TestDeveloperKeys_CreateAcceptsArraysAndHashesSecret(t *testing.T) {
	repo := new(mocks.MockDeveloperKeyRepository)
	var stored *models.DeveloperKey
	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.DeveloperKey")).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*models.DeveloperKey) }).
		Return(nil)
	app := devKeysApp(repo, 1)

	resp := testutil.MakeRequest(app, http.MethodPost, "/accounts/self/developer_keys", testutil.JSONBody(map[string]interface{}{
		"developer_key": map[string]interface{}{
			"name":           "Example App",
			"redirect_uris":  []string{"https://app.example.com/cb1", " https://app.example.com/cb2 "},
			"scopes":         []string{"url:GET|/api/v1/courses"},
			"require_scopes": true,
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)

	require.NotNil(t, stored)
	assert.Equal(t, uint(1), stored.AccountID)
	assert.Equal(t, "https://app.example.com/cb1\nhttps://app.example.com/cb2", stored.RedirectURIs)
	assert.Equal(t, `["url:GET|/api/v1/courses"]`, stored.Scopes)
	assert.True(t, stored.RequireScopes)

	// B191: only the hash is stored; the response carries the raw secret once.
	raw, _ := body["client_secret"].(string)
	require.NotEmpty(t, raw)
	assert.True(t, strings.HasPrefix(stored.ClientSecret, "sha256:"))
	assert.NotContains(t, stored.ClientSecret, raw)
	assert.True(t, service.VerifyClientSecret(stored, raw))
	assert.False(t, service.VerifyClientSecret(stored, raw+"x"))
}

// LTI 1.3 developer keys carry LTI Advantage service scopes (Canvas
// accepts them); scope validation must not reject them, while free-form
// text is still refused.
func TestDeveloperKeys_CreateAcceptsLTIAdvantageScopes(t *testing.T) {
	repo := new(mocks.MockDeveloperKeyRepository)
	var stored *models.DeveloperKey
	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.DeveloperKey")).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*models.DeveloperKey) }).
		Return(nil)
	app := devKeysApp(repo, 1)

	ltiScopes := []string{
		"https://purl.imsglobal.org/spec/lti-ags/scope/lineitem",
		"https://purl.imsglobal.org/spec/lti-ags/scope/score",
		"https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly",
	}
	resp := testutil.MakeRequest(app, http.MethodPost, "/accounts/self/developer_keys", testutil.JSONBody(map[string]interface{}{
		"developer_key": map[string]interface{}{
			"name":       "LTI Tool",
			"is_lti_key": true,
			"scopes":     ltiScopes,
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, stored)
	assert.Equal(t, service.ScopesToJSON(ltiScopes), stored.Scopes)

	resp = testutil.MakeRequest(app, http.MethodPost, "/accounts/self/developer_keys", testutil.JSONBody(map[string]interface{}{
		"developer_key": map[string]interface{}{"name": "Bad", "scopes": []string{"read:courses"}},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- B076: OAuth2 authorization requires explicit consent -----------------

func oauthFixture(keyAccount uint) (*mocks.MockDeveloperKeyRepository, *handlers.OAuth2Handler) {
	repo := new(mocks.MockDeveloperKeyRepository)
	repo.On("FindByClientID", mock.Anything, "dk_example").Return(&models.DeveloperKey{
		ID: 5, AccountID: keyAccount, Name: "Example App", ClientID: "dk_example",
		RedirectURI: "https://app.example.com/cb", WorkflowState: "active",
	}, nil)
	devKeySvc := service.NewDeveloperKeyService(repo)
	tokenSvc := service.NewAccessTokenService(new(mocks.MockAccessTokenRepository))
	h := handlers.NewOAuth2Handler(service.NewOAuth2Service(devKeySvc, tokenSvc), devKeySvc, tokenSvc, nil)
	h.SetFrontendURL("https://lms.example.com")
	return repo, h
}

func TestOAuth2Authorize_GetNeverIssuesCode(t *testing.T) {
	_, h := oauthFixture(1)
	app := testutil.SetupTestApp()
	// Even with a live session (the CSRF-able case), GET must not mint a code.
	app.Get("/login/oauth2/auth", callerStub(10, 1, ""), h.Authorize)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/login/oauth2/auth?client_id=dk_example&response_type=code&redirect_uri="+url.QueryEscape("https://app.example.com/cb")+"&state=s1", nil)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	assert.True(t, strings.HasPrefix(loc, "https://lms.example.com/login/oauth2/confirm?"), loc)
	assert.NotContains(t, loc, "code=")
	assert.NotContains(t, loc, "app.example.com/cb?")

	// An unregistered redirect_uri is refused outright, never redirected to.
	resp = testutil.MakeRequest(app, http.MethodGet,
		"/login/oauth2/auth?client_id=dk_example&response_type=code&redirect_uri="+url.QueryEscape("https://evil.example.net/cb"), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOAuth2Consent_RefusesClientFromAnotherTenant(t *testing.T) {
	_, h := oauthFixture(2) // key owned by tenant 2
	app := testutil.SetupTestApp()
	app.Use(callerStub(10, 1, "")) // user in tenant 1
	app.Get("/login/oauth2/auth/info", h.AuthorizeInfo)
	app.Post("/login/oauth2/auth", h.AuthorizePost)

	resp := testutil.MakeRequest(app, http.MethodGet,
		"/login/oauth2/auth/info?client_id=dk_example&redirect_uri="+url.QueryEscape("https://app.example.com/cb"), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = testutil.MakeRequest(app, http.MethodPost, "/login/oauth2/auth", testutil.JSONBody(map[string]interface{}{
		"client_id": "dk_example", "redirect_uri": "https://app.example.com/cb",
	}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestOAuth2Consent_ApproveAndDeny(t *testing.T) {
	_, h := oauthFixture(1)
	app := testutil.SetupTestApp()
	app.Use(callerStub(10, 1, ""))
	app.Post("/login/oauth2/auth", h.AuthorizePost)

	resp := testutil.MakeRequest(app, http.MethodPost, "/login/oauth2/auth", testutil.JSONBody(map[string]interface{}{
		"client_id": "dk_example", "redirect_uri": "https://app.example.com/cb", "state": "s1", "approve": true,
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	u, err := url.Parse(body["redirect_url"].(string))
	require.NoError(t, err)
	assert.Equal(t, "app.example.com", u.Host)
	assert.NotEmpty(t, u.Query().Get("code"))
	assert.Equal(t, "s1", u.Query().Get("state"))

	resp = testutil.MakeRequest(app, http.MethodPost, "/login/oauth2/auth", testutil.JSONBody(map[string]interface{}{
		"client_id": "dk_example", "redirect_uri": "https://app.example.com/cb", "state": "s1", "approve": false,
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ = testutil.ParseJSONMap(resp)
	u, _ = url.Parse(body["redirect_url"].(string))
	assert.Equal(t, "access_denied", u.Query().Get("error"))
	assert.Empty(t, u.Query().Get("code"))
}

// --- B184 / B193: refresh grant ---------------------------------------------

func TestOAuth2RefreshGrant_BindsClientAndKeepsRefreshToken(t *testing.T) {
	devRepo := new(mocks.MockDeveloperKeyRepository)
	devRepo.On("FindByClientID", mock.Anything, "dk_b").Return(&models.DeveloperKey{
		ID: 6, AccountID: 1, ClientID: "dk_b", ClientSecret: service.HashClientSecret("secret-b"), WorkflowState: "active",
	}, nil)
	devRepo.On("FindByClientID", mock.Anything, "dk_a").Return(&models.DeveloperKey{
		ID: 5, AccountID: 1, ClientID: "dk_a", ClientSecret: service.HashClientSecret("secret-a"), WorkflowState: "active",
	}, nil)
	tokenRepo := new(mocks.MockAccessTokenRepository)
	keyA := uint(5)
	refreshHash := "unused"
	tokenRepo.On("FindByRefreshToken", mock.Anything, mock.Anything).Return(&models.AccessToken{
		ID: 9, UserID: 10, DeveloperKeyID: &keyA, RefreshToken: &refreshHash, WorkflowState: "active",
	}, nil)
	tokenRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	devKeySvc := service.NewDeveloperKeyService(devRepo)
	tokenSvc := service.NewAccessTokenService(tokenRepo)
	h := handlers.NewOAuth2Handler(service.NewOAuth2Service(devKeySvc, tokenSvc), devKeySvc, tokenSvc, nil)
	app := testutil.SetupTestApp()
	app.Post("/login/oauth2/token", h.Token)

	// Client B presents client A's refresh token → refused.
	resp := testutil.MakeRequest(app, http.MethodPost, "/login/oauth2/token", testutil.JSONBody(map[string]string{
		"grant_type": "refresh_token", "refresh_token": "a-refresh", "client_id": "dk_b", "client_secret": "secret-b",
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	tokenRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)

	// Client A refreshes: new access token, no refresh_token rotation.
	resp = testutil.MakeRequest(app, http.MethodPost, "/login/oauth2/token", testutil.JSONBody(map[string]string{
		"grant_type": "refresh_token", "refresh_token": "a-refresh", "client_id": "dk_a", "client_secret": "secret-a",
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.NotEmpty(t, body["access_token"])
	_, hasRefresh := body["refresh_token"]
	assert.False(t, hasRefresh, "Canvas does not rotate/return refresh tokens on refresh")
	tokenRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	updated := tokenRepo.Calls[len(tokenRepo.Calls)-1].Arguments.Get(1).(*models.AccessToken)
	assert.Equal(t, &refreshHash, updated.RefreshToken)
}

// --- B190: "self" alias on the token endpoints --------------------------------

func TestAccessTokens_SelfAlias(t *testing.T) {
	tokenRepo := new(mocks.MockAccessTokenRepository)
	tokenRepo.On("ListByUserID", mock.Anything, uint(10), mock.Anything).Return(&repository.PaginatedResult[models.AccessToken]{
		Items: []models.AccessToken{}, TotalCount: 0, Page: 1, PerPage: 10,
	}, nil)
	h := handlers.NewAccessTokenHandler(service.NewAccessTokenService(tokenRepo))
	app := testutil.SetupTestApp()
	app.Use(callerStub(10, 1, ""), middleware.PaginationParams())
	app.Get("/users/:user_id/tokens", h.ListAccessTokens)

	resp := testutil.MakeRequest(app, http.MethodGet, "/users/self/tokens", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, "/users/11/tokens", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// B085: PAT scopes are enforced, so only enforceable (Canvas url:) scopes
// are accepted at creation.
func TestAccessTokens_CreateRejectsUnenforceableScopes(t *testing.T) {
	tokenRepo := new(mocks.MockAccessTokenRepository)
	tokenRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	h := handlers.NewAccessTokenHandler(service.NewAccessTokenService(tokenRepo))
	app := testutil.SetupTestApp()
	app.Use(callerStub(10, 1, ""))
	app.Post("/users/:user_id/tokens", h.CreateAccessToken)

	resp := testutil.MakeRequest(app, http.MethodPost, "/users/self/tokens", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"purpose": "CI", "scopes": []string{"read:courses"}},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	tokenRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	resp = testutil.MakeRequest(app, http.MethodPost, "/users/self/tokens", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"purpose": "CI", "scopes": []string{"url:GET|/api/v1/courses"}},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, []interface{}{"url:GET|/api/v1/courses"}, body["scopes"])
}

// --- B185 / B084: external tools ----------------------------------------------

func externalToolsApp(toolRepo *mocks.MockContextExternalToolRepository, devRepo *mocks.MockDeveloperKeyRepository, enrollmentType string) *fiber.App {
	h := handlers.NewExternalToolHandler(service.NewExternalToolService(toolRepo, devRepo), service.NewDeveloperKeyService(devRepo))
	app := testutil.SetupTestApp()
	app.Use(callerStub(10, 1, enrollmentType))
	app.Get("/courses/:course_id/external_tools/:id", h.GetExternalTool)
	app.Put("/courses/:course_id/external_tools/:id", h.UpdateExternalTool)
	app.Delete("/courses/:course_id/external_tools/:id", h.DeleteExternalTool)
	app.Post("/courses/:course_id/external_tools", h.CreateExternalTool)
	return app
}

func TestExternalTools_ParentTieRefusesOtherCoursesTool(t *testing.T) {
	toolRepo := new(mocks.MockContextExternalToolRepository)
	toolRepo.On("FindByID", mock.Anything, uint(55), uint(1)).Return(&models.ContextExternalTool{
		ID: 55, ContextType: "Course", ContextID: 20, DeveloperKeyID: 5, Name: "Other", WorkflowState: "active",
		ConsumerKey: "ck",
	}, nil)
	app := externalToolsApp(toolRepo, new(mocks.MockDeveloperKeyRepository), "TeacherEnrollment")

	for _, tc := range []struct {
		method string
		body   interface{}
	}{
		{http.MethodGet, nil},
		{http.MethodPut, map[string]interface{}{"external_tool": map[string]interface{}{"url": "https://phish.example.net"}}},
		{http.MethodDelete, nil},
	} {
		var body = testutil.JSONBody(tc.body)
		if tc.body == nil {
			body = nil
		}
		resp := testutil.MakeRequest(app, tc.method, "/courses/10/external_tools/55", body)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, tc.method)
	}
	toolRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)

	// Same course works; a student does not see the LTI 1.1 consumer key.
	studentApp := externalToolsApp(toolRepo, new(mocks.MockDeveloperKeyRepository), "StudentEnrollment")
	resp := testutil.MakeRequest(studentApp, http.MethodGet, "/courses/20/external_tools/55", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	_, hasKey := body["consumer_key"]
	assert.False(t, hasKey)
}

func TestExternalTools_CreateAcceptsFormShapes(t *testing.T) {
	toolRepo := new(mocks.MockContextExternalToolRepository)
	devRepo := new(mocks.MockDeveloperKeyRepository)
	devRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.DeveloperKey{ID: 5, AccountID: 1}, nil)
	devRepo.On("FindByID", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	var stored *models.ContextExternalTool
	toolRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.ContextExternalTool")).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*models.ContextExternalTool) }).Return(nil)
	app := externalToolsApp(toolRepo, devRepo, "TeacherEnrollment")

	// What the External Tools page used to send: string id + object fields.
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/10/external_tools", testutil.JSONBody(map[string]interface{}{
		"external_tool": map[string]interface{}{
			"name": "Example Tool", "developer_key_id": "5", "custom_fields": map[string]string{"unit": "3"},
		},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, stored)
	assert.Equal(t, uint(5), stored.DeveloperKeyID)
	assert.JSONEq(t, `{"unit":"3"}`, stored.CustomFields)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, map[string]interface{}{"unit": "3"}, body["custom_fields"])

	// Missing developer key → 400, not 500.
	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/10/external_tools", testutil.JSONBody(map[string]interface{}{
		"external_tool": map[string]interface{}{"name": "No Key"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// A developer key of another tenant → 400, not 500.
	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/10/external_tools", testutil.JSONBody(map[string]interface{}{
		"external_tool": map[string]interface{}{"name": "Foreign Key", "developer_key_id": 77},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
