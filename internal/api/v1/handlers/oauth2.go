package handlers

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
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
	// tokenBlacklist revokes the browser session JWT on
	// DELETE /login/oauth2/token?expire_sessions=1 (nil: cookie cleared only).
	tokenBlacklist *service.TokenBlacklist
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

// SetTokenBlacklist wires session-JWT revocation for the logout endpoint.
func (h *OAuth2Handler) SetTokenBlacklist(bl *service.TokenBlacklist) {
	h.tokenBlacklist = bl
}

// oauthConsentPath is the SPA route that renders the consent screen.
const oauthConsentPath = "/login/oauth2/confirm"

func splitScopes(scope string) []string {
	return strings.Fields(scope)
}

// oauthError writes an RFC 6749 §5.2 error body. The Paper/Canvas
// "errors" array is kept alongside so existing clients still find a
// message.
func oauthError(c *fiber.Ctx, status int, code, description string) error {
	if status == fiber.StatusUnauthorized {
		c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="Paper LMS OAuth 2.0"`)
	}
	return c.Status(status).JSON(fiber.Map{
		"error":             code,
		"error_description": description,
		"errors":            []fiber.Map{{"message": description}},
	})
}

// requestedScope returns the scope parameter (Canvas also accepts
// "scopes").
func requestedScope(c *fiber.Ctx) string {
	if s := c.Query("scope"); s != "" {
		return s
	}
	return c.Query("scopes")
}

// Authorize is the OAuth2 authorization endpoint.
// GET /login/oauth2/auth and GET /api/v1/login/oauth2/auth (PUBLIC)
//
// B076: this endpoint NEVER issues a code. A GET is reachable cross-site
// with the Lax session cookie attached and is CSRF-exempt, so issuing a
// code here let any page that linked a logged-in user to it harvest an
// API token for that user. Instead the request is validated (client
// exists, is active, the redirect_uri is registered — never redirect to
// an unvalidated URI) and the browser is sent to the SPA consent screen,
// which requires a session, shows the app + scopes, and posts the user's
// explicit decision to AuthorizePost (CSRF-protected).
//
// G136: once the client and redirect_uri are known good, request errors
// go back to the client the RFC 6749 §4.1.2.1 way (error= on the
// redirect_uri), as Canvas does: unsupported_response_type, invalid_scope
// for a require_scopes key, and invalid_request for a malformed PKCE
// challenge. PKCE parameters (code_challenge, code_challenge_method=S256)
// are carried through the consent screen to the code.
func (h *OAuth2Handler) Authorize(c *fiber.Ctx) error {
	clientID := c.Query("client_id")
	responseType := c.Query("response_type")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")

	if clientID == "" {
		return responses.BadRequest(c, "client_id is required")
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

	clientErr := func(code, description string) error {
		target, err := buildRedirectURLWithParams(redirectURI, map[string]string{
			"error": code, "error_description": description, "state": state,
		})
		if err != nil {
			return responses.BadRequest(c, description)
		}
		return c.Redirect(target, fiber.StatusFound)
	}

	if responseType != "code" {
		return clientErr("unsupported_response_type", "Only response_type=code is permitted")
	}
	scope := requestedScope(c)
	if devKey.RequireScopes {
		requested := splitScopes(scope)
		if len(requested) == 0 {
			return clientErr("invalid_scope", "This application must request scopes")
		}
		if _, err := h.oauth2Service.ValidateScopes(requested, service.ParseScopes(devKey.Scopes)); err != nil {
			return clientErr("invalid_scope", "A requested scope is invalid, unknown, malformed, or exceeds the scope granted to this application")
		}
	}
	pkce := service.PKCEChallenge{Challenge: c.Query("code_challenge"), Method: c.Query("code_challenge_method")}
	if err := pkce.Validate(); err != nil {
		return clientErr("invalid_request", err.Error())
	}

	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	if scope != "" {
		q.Set("scope", scope)
	}
	if state != "" {
		q.Set("state", state)
	}
	if pkce.Challenge != "" {
		q.Set("code_challenge", pkce.Challenge)
		q.Set("code_challenge_method", pkce.Method)
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
		"pkce":         c.Query("code_challenge") != "",
	})
}

// AuthorizePost records the user's explicit consent decision.
// POST /api/v1/login/oauth2/auth (authenticated + CSRF)
//
// Body: {client_id, redirect_uri, scope, state, approve, code_challenge,
// code_challenge_method}. approve=false (deny) sends the client back with
// error=access_denied. The response is JSON {redirect_url} so the SPA can
// navigate the top-level window: a fetch cannot usefully follow a
// cross-origin 302.
func (h *OAuth2Handler) AuthorizePost(c *fiber.Ctx) error {
	var input struct {
		ClientID            string `json:"client_id"`
		RedirectURI         string `json:"redirect_uri"`
		Scope               string `json:"scope"`
		State               string `json:"state"`
		Approve             *bool  `json:"approve"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
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

	code, err := h.oauth2Service.GenerateAuthorizationCodePKCE(
		c.Context(),
		userID,
		accountID,
		input.ClientID,
		input.RedirectURI,
		splitScopes(input.Scope),
		service.PKCEChallenge{Challenge: input.CodeChallenge, Method: input.CodeChallengeMethod},
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

// tokenRequest is the token endpoint's input, from a form or JSON body.
type tokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	Code         string `json:"code" form:"code"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
}

// basicClientCredentials reads client_id / client_secret from an HTTP
// Basic Authorization header (RFC 6749 §2.3.1), which Canvas accepts.
func basicClientCredentials(c *fiber.Ctx) (string, string, bool) {
	raw, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Basic ")
	if !ok {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return "", "", false
	}
	id, secret, found := strings.Cut(string(decoded), ":")
	if !found {
		return "", "", false
	}
	id, _ = url.QueryUnescape(id)
	secret, _ = url.QueryUnescape(secret)
	return id, secret, true
}

// Token exchanges an authorization code or refresh token for access tokens.
// POST /login/oauth2/token and POST /api/v1/login/oauth2/token (PUBLIC)
//
// Supports two grant types:
//   - authorization_code: exchange a code for access_token + refresh_token
//     (the default when grant_type is omitted but a code is sent, as in
//     Canvas). With PKCE, code_verifier is required and client_secret may
//     be omitted (public clients).
//   - refresh_token: mint a new access_token (the refresh token is kept)
//
// Client credentials may come in the body or an HTTP Basic header. Errors
// use RFC 6749 §5.2 bodies ({error, error_description}).
func (h *OAuth2Handler) Token(c *fiber.Ctx) error {
	var in tokenRequest
	if strings.HasPrefix(string(c.Request().Header.ContentType()), fiber.MIMEApplicationJSON) {
		if len(c.Body()) > 0 {
			if err := c.BodyParser(&in); err != nil {
				return oauthError(c, fiber.StatusBadRequest, "invalid_request", "Malformed request body")
			}
		}
	} else {
		in = tokenRequest{
			GrantType:    c.FormValue("grant_type"),
			Code:         c.FormValue("code"),
			ClientID:     c.FormValue("client_id"),
			ClientSecret: c.FormValue("client_secret"),
			RedirectURI:  c.FormValue("redirect_uri"),
			RefreshToken: c.FormValue("refresh_token"),
			CodeVerifier: c.FormValue("code_verifier"),
		}
	}
	if id, secret, ok := basicClientCredentials(c); ok {
		if in.ClientID == "" {
			in.ClientID = id
		}
		if in.ClientSecret == "" {
			in.ClientSecret = secret
		}
	}
	if in.GrantType == "" && in.Code != "" {
		in.GrantType = "authorization_code"
	}

	switch in.GrantType {
	case "authorization_code":
		return h.handleAuthorizationCodeGrant(c, in)
	case "refresh_token":
		return h.handleRefreshTokenGrant(c, in.RefreshToken, in.ClientID, in.ClientSecret)
	default:
		return oauthError(c, fiber.StatusBadRequest, "unsupported_grant_type", "Unsupported grant_type. Use 'authorization_code' or 'refresh_token'")
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
func (h *OAuth2Handler) handleAuthorizationCodeGrant(c *fiber.Ctx, in tokenRequest) error {
	if in.Code == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "code is required")
	}
	if in.ClientID == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "client_id is required")
	}
	if in.ClientSecret == "" && in.CodeVerifier == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "client_secret is required (or use PKCE with code_verifier)")
	}
	if in.RedirectURI == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "redirect_uri is required")
	}

	token, rawAccessToken, rawRefreshToken, err := h.oauth2Service.ExchangeCodePKCE(
		c.Context(),
		in.Code,
		in.ClientID,
		in.ClientSecret,
		in.RedirectURI,
		in.CodeVerifier,
	)
	if err != nil {
		if errors.Is(err, service.ErrOAuthInvalidClient) {
			return oauthError(c, fiber.StatusUnauthorized, "invalid_client", "Invalid client credentials")
		}
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", err.Error())
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
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "refresh_token is required")
	}
	if clientID == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "client_id is required")
	}
	if clientSecret == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "client_secret is required")
	}

	// Validate client credentials (constant-time; hashed at rest, B191)
	devKey, err := h.devKeyService.GetByClientID(c.Context(), clientID)
	if err != nil || !h.devKeyService.VerifyClientSecret(devKey, clientSecret) {
		return oauthError(c, fiber.StatusUnauthorized, "invalid_client", "Invalid client credentials")
	}
	if devKey.WorkflowState != "active" {
		return oauthError(c, fiber.StatusUnauthorized, "invalid_client", "Developer key is not active")
	}

	token, rawAccessToken, err := h.tokenService.RefreshOAuthToken(c.Context(), refreshToken, devKey.ID)
	if err != nil {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", err.Error())
	}

	return c.JSON(fiber.Map{
		"access_token": rawAccessToken,
		"token_type":   "Bearer",
		"expires_in":   tokenExpiresIn(token),
		"user":         h.tokenUserInfo(c, token.UserID),
	})
}

// Logout revokes the access token the request authenticated with.
// DELETE /login/oauth2/token and DELETE /api/v1/login/oauth2/token
// (Bearer access token)
//
// Canvas parity (G136): only a request made WITH an access token can
// delete it (a cookie session gets 400). expire_sessions=1 also ends the
// caller's browser session (the session cookie is revoked and cleared).
func (h *OAuth2Handler) Logout(c *fiber.Ctx) error {
	tokenID, ok := c.Locals("access_token_id").(uint)
	if !ok || tokenID == 0 {
		return responses.BadRequest(c, "can't delete OAuth access token when not using an OAuth access token")
	}
	userID, _ := c.Locals("user_id").(uint)
	if err := h.tokenService.RevokeToken(c.Context(), tokenID, userID); err != nil {
		return responses.InternalError(c, "Could not revoke the access token")
	}
	if q := strings.ToLower(c.Query("expire_sessions")); q == "1" || q == "true" {
		if session := c.Cookies("paper_session"); session != "" && h.tokenBlacklist != nil {
			h.tokenBlacklist.Revoke(session, time.Now().Add(24*time.Hour))
		}
		c.Cookie(&fiber.Cookie{
			Name:     "paper_session",
			Value:    "",
			Path:     "/",
			HTTPOnly: true,
			MaxAge:   -1,
			Expires:  time.Now().Add(-1 * time.Hour),
		})
	}
	return c.JSON(fiber.Map{})
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
