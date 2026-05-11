package handlers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type OAuth2Handler struct {
	oauth2Service *service.OAuth2Service
	devKeyService *service.DeveloperKeyService
	tokenService  *service.AccessTokenService
	userService   *service.UserService
}

func NewOAuth2Handler(
	oauth2Service *service.OAuth2Service,
	devKeyService *service.DeveloperKeyService,
	tokenService *service.AccessTokenService,
	userService *service.UserService,
) *OAuth2Handler {
	return &OAuth2Handler{
		oauth2Service: oauth2Service,
		devKeyService: devKeyService,
		tokenService:  tokenService,
		userService:   userService,
	}
}

// Authorize handles the OAuth2 authorization endpoint.
// GET /api/v1/login/oauth2/auth
//
// If the user is already authenticated (has a valid Bearer token), this
// generates an authorization code and redirects to the client's redirect_uri.
// If the user is not authenticated, it returns a JSON response describing the
// authorization request so the frontend can display a consent screen.
func (h *OAuth2Handler) Authorize(c *fiber.Ctx) error {
	clientID := c.Query("client_id")
	responseType := c.Query("response_type")
	redirectURI := c.Query("redirect_uri")
	scope := c.Query("scope")
	state := c.Query("state")

	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}
	if responseType != "code" {
		return responses.BadRequest(c, "response_type must be 'code'")
	}
	if redirectURI == "" {
		return responses.BadRequest(c, "redirect_uri is required")
	}

	// Validate that the developer key exists and is active
	devKey, err := h.devKeyService.GetByClientID(c.Context(), clientID)
	if err != nil {
		return responses.BadRequest(c, "Invalid client_id")
	}
	if devKey.WorkflowState != "active" {
		return responses.BadRequest(c, "Developer key is not active")
	}

	// Validate redirect URI
	if !h.devKeyService.ValidateRedirectURI(devKey, redirectURI) {
		return responses.BadRequest(c, "Invalid redirect_uri")
	}

	// Parse scopes
	var scopes []string
	if scope != "" {
		scopes = strings.Split(scope, " ")
	}

	// Check if the user is authenticated
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		// User is not authenticated; return authorization info for the frontend
		return c.JSON(fiber.Map{
			"authorize_url": fmt.Sprintf("/api/v1/login/oauth2/auth?client_id=%s&response_type=code&redirect_uri=%s&scope=%s&state=%s",
				url.QueryEscape(clientID),
				url.QueryEscape(redirectURI),
				url.QueryEscape(scope),
				url.QueryEscape(state),
			),
			"client_name": devKey.Name,
			"scopes":      scopes,
		})
	}

	// User is authenticated; generate auth code and redirect
	code, err := h.oauth2Service.GenerateAuthorizationCode(
		c.Context(),
		userID,
		clientID,
		redirectURI,
		scopes,
		state,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Build the redirect URL with code and state
	redirectURL, err := buildRedirectURL(redirectURI, code, state)
	if err != nil {
		return responses.InternalError(c, "Could not build redirect URL")
	}

	return c.Redirect(redirectURL, fiber.StatusFound)
}

// AuthorizePost handles explicit user consent in the OAuth2 flow.
// POST /api/v1/login/oauth2/auth
//
// The user has explicitly consented to the authorization. This endpoint
// generates an authorization code and redirects to the client's redirect_uri.
func (h *OAuth2Handler) AuthorizePost(c *fiber.Ctx) error {
	var input struct {
		ClientID    string `json:"client_id"`
		RedirectURI string `json:"redirect_uri"`
		Scope       string `json:"scope"`
		State       string `json:"state"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ClientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}
	if input.RedirectURI == "" {
		return responses.BadRequest(c, "redirect_uri is required")
	}

	// Validate that the developer key exists and is active
	devKey, err := h.devKeyService.GetByClientID(c.Context(), input.ClientID)
	if err != nil {
		return responses.BadRequest(c, "Invalid client_id")
	}
	if devKey.WorkflowState != "active" {
		return responses.BadRequest(c, "Developer key is not active")
	}

	// Validate redirect URI
	if !h.devKeyService.ValidateRedirectURI(devKey, input.RedirectURI) {
		return responses.BadRequest(c, "Invalid redirect_uri")
	}

	// The user must be authenticated for the POST consent flow
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}

	var scopes []string
	if input.Scope != "" {
		scopes = strings.Split(input.Scope, " ")
	}

	code, err := h.oauth2Service.GenerateAuthorizationCode(
		c.Context(),
		userID,
		input.ClientID,
		input.RedirectURI,
		scopes,
		input.State,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	redirectURL, err := buildRedirectURL(input.RedirectURI, code, input.State)
	if err != nil {
		return responses.InternalError(c, "Could not build redirect URL")
	}

	return c.Redirect(redirectURL, fiber.StatusFound)
}

// Token exchanges an authorization code or refresh token for access tokens.
// POST /api/v1/login/oauth2/token (PUBLIC - no auth required)
//
// Supports two grant types:
//   - authorization_code: exchange a code for access_token + refresh_token
//   - refresh_token: rotate tokens using a valid refresh_token
func (h *OAuth2Handler) Token(c *fiber.Ctx) error {
	grantType := c.FormValue("grant_type")
	if grantType == "" {
		// Also try JSON body
		var body struct {
			GrantType    string `json:"grant_type"`
			Code         string `json:"code"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			RedirectURI  string `json:"redirect_uri"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.BodyParser(&body); err == nil {
			grantType = body.GrantType
			if grantType == "authorization_code" {
				return h.handleAuthorizationCodeGrant(c, body.Code, body.ClientID, body.ClientSecret, body.RedirectURI)
			}
			if grantType == "refresh_token" {
				return h.handleRefreshTokenGrant(c, body.RefreshToken, body.ClientID, body.ClientSecret)
			}
		}
	}

	switch grantType {
	case "authorization_code":
		code := c.FormValue("code")
		clientID := c.FormValue("client_id")
		clientSecret := c.FormValue("client_secret")
		redirectURI := c.FormValue("redirect_uri")
		return h.handleAuthorizationCodeGrant(c, code, clientID, clientSecret, redirectURI)

	case "refresh_token":
		refreshToken := c.FormValue("refresh_token")
		clientID := c.FormValue("client_id")
		clientSecret := c.FormValue("client_secret")
		return h.handleRefreshTokenGrant(c, refreshToken, clientID, clientSecret)

	default:
		return responses.BadRequest(c, "Unsupported grant_type. Use 'authorization_code' or 'refresh_token'")
	}
}

// handleAuthorizationCodeGrant exchanges an authorization code for tokens.
func (h *OAuth2Handler) handleAuthorizationCodeGrant(c *fiber.Ctx, code, clientID, clientSecret, redirectURI string) error {
	if code == "" {
		return responses.BadRequest(c, "code is required")
	}
	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}
	if clientSecret == "" {
		return responses.BadRequest(c, "client_secret is required")
	}
	if redirectURI == "" {
		return responses.BadRequest(c, "redirect_uri is required")
	}

	token, rawAccessToken, rawRefreshToken, err := h.oauth2Service.ExchangeCode(
		c.Context(),
		code,
		clientID,
		clientSecret,
		redirectURI,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Calculate expires_in from the token's ExpiresAt
	expiresIn := 3600
	if token.ExpiresAt != nil {
		remaining := int(token.ExpiresAt.Sub(token.CreatedAt).Seconds())
		if remaining > 0 {
			expiresIn = remaining
		}
	}

	// Look up user info
	userInfo := fiber.Map{"id": token.UserID}
	if h.userService != nil {
		user, userErr := h.userService.GetByID(c.Context(), token.UserID)
		if userErr == nil {
			userInfo["id"] = user.ID
			userInfo["name"] = user.Name
		}
	}

	return c.JSON(fiber.Map{
		"access_token":  rawAccessToken,
		"token_type":    "Bearer",
		"refresh_token": rawRefreshToken,
		"expires_in":    expiresIn,
		"user":          userInfo,
	})
}

// handleRefreshTokenGrant rotates tokens using a refresh token.
func (h *OAuth2Handler) handleRefreshTokenGrant(c *fiber.Ctx, refreshToken, clientID, clientSecret string) error {
	if refreshToken == "" {
		return responses.BadRequest(c, "refresh_token is required")
	}
	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}
	if clientSecret == "" {
		return responses.BadRequest(c, "client_secret is required")
	}

	// Validate client credentials
	devKey, err := h.devKeyService.GetByClientID(c.Context(), clientID)
	if err != nil {
		return responses.BadRequest(c, "Invalid client_id")
	}
	if devKey.ClientSecret != clientSecret {
		return responses.BadRequest(c, "Invalid client_secret")
	}
	if devKey.WorkflowState != "active" {
		return responses.BadRequest(c, "Developer key is not active")
	}

	token, rawAccessToken, rawRefreshToken, err := h.tokenService.RefreshOAuthToken(c.Context(), refreshToken)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Calculate expires_in
	expiresIn := 3600
	if token.ExpiresAt != nil {
		remaining := int(token.ExpiresAt.Sub(token.CreatedAt).Seconds())
		if remaining > 0 {
			expiresIn = remaining
		}
	}

	// Look up user info
	userInfo := fiber.Map{"id": token.UserID}
	if h.userService != nil {
		user, userErr := h.userService.GetByID(c.Context(), token.UserID)
		if userErr == nil {
			userInfo["id"] = user.ID
			userInfo["name"] = user.Name
		}
	}

	return c.JSON(fiber.Map{
		"access_token":  rawAccessToken,
		"token_type":    "Bearer",
		"refresh_token": rawRefreshToken,
		"expires_in":    expiresIn,
		"user":          userInfo,
	})
}

// buildRedirectURL appends the authorization code and optional state to the
// client's redirect URI.
func buildRedirectURL(redirectURI, code, state string) (string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
