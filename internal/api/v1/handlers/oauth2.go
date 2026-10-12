package handlers

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type OAuth2Handler struct {
	oauth2Service *service.OAuth2Service
	devKeyService *service.DeveloperKeyService
	tokenService  *service.AccessTokenService
	userService   *service.UserService
	// frontendURL is the SPA origin (cfg.FrontendURL). The GET
	// authorization endpoint redirects the browser to the SPA consent
	// page there. Empty means same-origin (relative redirect).
	frontendURL string
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

// SetFrontendURL configures the SPA origin used for the consent redirect.
func (h *OAuth2Handler) SetFrontendURL(frontendURL string) {
	h.frontendURL = strings.TrimSuffix(frontendURL, "/")
}

// oauthConsentPath is the SPA route that renders the consent screen.
const oauthConsentPath = "/login/oauth2/confirm"

func splitScopes(scope string) []string {
	return strings.Fields(scope)
}

// Authorize is the OAuth2 authorization endpoint.
// GET /api/v1/login/oauth2/auth (PUBLIC)
//
// B076: this endpoint NEVER issues a code. A GET is reachable cross-site
// with the Lax session cookie attached and is CSRF-exempt, so issuing a
// code here let any page that linked a logged-in user to it harvest an
// API token for that user. Instead the request is validated (client
// exists, is active, the redirect_uri is registered — never redirect to
// an unvalidated URI) and the browser is sent to the SPA consent screen,
// which requires a session, shows the app + scopes, and posts the user's
// explicit decision to AuthorizePost (CSRF-protected).
func (h *OAuth2Handler) Authorize(c *fiber.Ctx) error {
	clientID := c.Query("client_id")
	responseType := c.Query("response_type")
	redirectURI := c.Query("redirect_uri")

	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
	}
	if responseType != "code" {
		return responses.BadRequest(c, "response_type must be 'code'")
	}
	if redirectURI == "" {
		return responses.BadRequest(c, "redirect_uri is required")
	}

	devKey, err := h.devKeyService.GetByClientID(c.Context(), clientID)
	if err != nil || devKey.WorkflowState != "active" {
		return responses.BadRequest(c, "Invalid client_id")
	}
	if !h.devKeyService.ValidateRedirectURI(devKey, redirectURI) {
		return responses.BadRequest(c, "Invalid redirect_uri")
	}

	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	if scope := c.Query("scope"); scope != "" {
		q.Set("scope", scope)
	}
	if state := c.Query("state"); state != "" {
		q.Set("state", state)
	}
	return c.Redirect(h.frontendURL+oauthConsentPath+"?"+q.Encode(), fiber.StatusFound)
}

// AuthorizeInfo returns what the consent screen needs to show.
// GET /api/v1/login/oauth2/auth/info (authenticated)
//
// The client must belong to the caller's tenant; anything else is the
// existence-leak 404.
func (h *OAuth2Handler) AuthorizeInfo(c *fiber.Ctx) error {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	if clientID == "" || redirectURI == "" {
		return responses.BadRequest(c, "client_id and redirect_uri are required")
	}

	devKey, err := h.oauth2Service.ResolveClientForUser(c.Context(), clientID, redirectURI, callerAccountID(c))
	if err != nil {
		if errors.Is(err, service.ErrOAuthClientNotFound) {
			return responses.NotFound(c, "developer key")
		}
		return responses.BadRequest(c, "Invalid redirect_uri")
	}

	return c.JSON(fiber.Map{
		"client_id":    clientID,
		"client_name":  devKey.Name,
		"icon_url":     devKey.Icon,
		"redirect_uri": redirectURI,
		"scopes":       splitScopes(c.Query("scope")),
		"state":        c.Query("state"),
	})
}

// AuthorizePost records the user's explicit consent decision.
// POST /api/v1/login/oauth2/auth (authenticated + CSRF)
//
// Body: {client_id, redirect_uri, scope, state, approve}. approve=false
// (deny) sends the client back with error=access_denied. The response is
// JSON {redirect_url} so the SPA can navigate the top-level window: a
// fetch cannot usefully follow a cross-origin 302.
func (h *OAuth2Handler) AuthorizePost(c *fiber.Ctx) error {
	var input struct {
		ClientID    string `json:"client_id"`
		RedirectURI string `json:"redirect_uri"`
		Scope       string `json:"scope"`
		State       string `json:"state"`
		Approve     *bool  `json:"approve"`
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

	// The user must be authenticated for the POST consent flow
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.Unauthorized(c)
	}
	accountID := callerAccountID(c)

	// Validate the client for this user's tenant BEFORE building any
	// redirect, so a deny can never bounce to an unvalidated URI.
	if _, err := h.oauth2Service.ResolveClientForUser(c.Context(), input.ClientID, input.RedirectURI, accountID); err != nil {
		if errors.Is(err, service.ErrOAuthClientNotFound) {
			return responses.NotFound(c, "developer key")
		}
		return responses.BadRequest(c, "Invalid redirect_uri")
	}

	if input.Approve != nil && !*input.Approve {
		redirectURL, err := buildRedirectURLWithParams(input.RedirectURI, map[string]string{
			"error": "access_denied",
			"state": input.State,
		})
		if err != nil {
			return responses.InternalError(c, "Could not build redirect URL")
		}
		return c.JSON(fiber.Map{"redirect_url": redirectURL})
	}

	code, err := h.oauth2Service.GenerateAuthorizationCode(
		c.Context(),
		userID,
		accountID,
		input.ClientID,
		input.RedirectURI,
		splitScopes(input.Scope),
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	redirectURL, err := buildRedirectURL(input.RedirectURI, code, input.State)
	if err != nil {
		return responses.InternalError(c, "Could not build redirect URL")
	}

	return c.JSON(fiber.Map{"redirect_url": redirectURL})
}

// Token exchanges an authorization code or refresh token for access tokens.
// POST /api/v1/login/oauth2/token (PUBLIC - no auth required)
//
// Supports two grant types:
//   - authorization_code: exchange a code for access_token + refresh_token
//   - refresh_token: mint a new access_token (the refresh token is kept)
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

// tokenExpiresIn returns the remaining lifetime of an access token in
// seconds (defaulting to the standard TTL).
func tokenExpiresIn(token *models.AccessToken) int {
	if token.ExpiresAt != nil {
		if remaining := int(time.Until(*token.ExpiresAt).Seconds()); remaining > 0 {
			return remaining
		}
	}
	return int(service.OAuthAccessTokenTTL.Seconds())
}

// tokenUserInfo builds the Canvas-style "user" object of a token response.
func (h *OAuth2Handler) tokenUserInfo(c *fiber.Ctx, userID uint) fiber.Map {
	userInfo := fiber.Map{"id": userID}
	if h.userService != nil {
		// AUTH-INTERNAL: OAuth2 token-grant response. The token row IS
		// the identity binding; the OAuth2 endpoint runs without a
		// session cookie / account_id Locals. accountID=0 is correct.
		user, userErr := h.userService.GetByID(c.Context(), userID, 0)
		if userErr == nil {
			userInfo["id"] = user.ID
			userInfo["name"] = user.Name
		}
	}
	return userInfo
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

	return c.JSON(fiber.Map{
		"access_token":  rawAccessToken,
		"token_type":    "Bearer",
		"refresh_token": rawRefreshToken,
		"expires_in":    tokenExpiresIn(token),
		"user":          h.tokenUserInfo(c, token.UserID),
	})
}

// handleRefreshTokenGrant mints a new access token from a refresh token.
//
// B184: the refresh token must belong to the authenticating client.
// B193: Canvas parity — the refresh token is not rotated, and the response
// carries no refresh_token (clients keep using the original one).
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

	// Validate client credentials (constant-time; hashed at rest, B191)
	devKey, err := h.devKeyService.GetByClientID(c.Context(), clientID)
	if err != nil {
		return responses.BadRequest(c, "Invalid client_id")
	}
	if !h.devKeyService.VerifyClientSecret(devKey, clientSecret) {
		return responses.BadRequest(c, "Invalid client_secret")
	}
	if devKey.WorkflowState != "active" {
		return responses.BadRequest(c, "Developer key is not active")
	}

	token, rawAccessToken, err := h.tokenService.RefreshOAuthToken(c.Context(), refreshToken, devKey.ID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"access_token": rawAccessToken,
		"token_type":   "Bearer",
		"expires_in":   tokenExpiresIn(token),
		"user":         h.tokenUserInfo(c, token.UserID),
	})
}

// buildRedirectURL appends the authorization code and optional state to the
// client's redirect URI.
func buildRedirectURL(redirectURI, code, state string) (string, error) {
	return buildRedirectURLWithParams(redirectURI, map[string]string{"code": code, "state": state})
}

// buildRedirectURLWithParams appends the non-empty params to redirectURI's
// query string.
func buildRedirectURLWithParams(redirectURI string, params map[string]string) (string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
