package handlers

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/gofiber/fiber/v2"
)

type AccessTokenHandler struct {
	tokenService *service.AccessTokenService
	// userRepo backs the admin path (G143): an account admin may list,
	// view and delete the tokens of users in their own tenant. Nil means
	// nobody but the owner can see a user's tokens.
	userRepo repository.UserRepository
}

func NewAccessTokenHandler(tokenService *service.AccessTokenService) *AccessTokenHandler {
	return &AccessTokenHandler{tokenService: tokenService}
}

// SetUserRepo enables admin token management (G143).
func (h *AccessTokenHandler) SetUserRepo(r repository.UserRepository) *AccessTokenHandler {
	h.userRepo = r
	return h
}

// accessTokenToJSON serializes an access token for API responses.
// The raw token value and refresh token are NEVER included here.
func accessTokenToJSON(token *models.AccessToken, callerID uint) fiber.Map {
	personal := service.IsPersonalAccessToken(token)
	state := token.WorkflowState
	if service.AccessTokenExpired(token) {
		state = "expired"
	}
	m := fiber.Map{
		"id":                      token.ID,
		"user_id":                 token.UserID,
		"purpose":                 token.Purpose,
		"token_hint":              token.TokenHint,
		"scopes":                  service.ParseScopes(token.Scopes),
		"expires_at":              token.ExpiresAt,
		"last_used_at":            token.LastUsedAt,
		"created_at":              token.CreatedAt,
		"workflow_state":          state,
		"kind":                    "oauth",
		"app_name":                nil,
		"can_manually_regenerate": personal && token.UserID == callerID,
	}
	if personal {
		m["kind"] = "personal"
	} else {
		// An OAuth token's expires_at is its hourly access-token expiry,
		// renewed by the app's refresh token, not a permanent expiry.
		m["expires_at"] = nil
		if token.DeveloperKey != nil {
			m["app_name"] = token.DeveloperKey.Name
		}
	}
	return m
}

// tokenOwner resolves the URL :user_id ("self" or a numeric id — B190) to
// the token owner and checks the caller may act on that user's tokens:
//
//   - the user themself may do anything;
//   - with allowAdmin, an account admin may act on a user of their own
//     tenant (a super admin on any user). Another tenant's user is a 404.
//
// Returns (ownerID, wrote); wrote==true means a response was already
// written and the caller must `return nil`.
func (h *AccessTokenHandler) tokenOwner(c *fiber.Ctx, allowAdmin bool, forbiddenMsg string) (uint, bool) {
	currentUserID, ok := c.Locals("user_id").(uint)
	if !ok || currentUserID == 0 {
		_ = responses.Unauthorized(c)
		return 0, true
	}
	raw := c.Params("user_id")
	if raw == "self" {
		return currentUserID, false
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		_ = responses.BadRequest(c, "Invalid user ID")
		return 0, true
	}
	if uint(id) == currentUserID {
		return currentUserID, false
	}
	if allowAdmin && h.userRepo != nil {
		switch middleware.EffectiveRole(c, h.userRepo) {
		case middleware.RoleSuperAdmin:
			// AUTH-INTERNAL: a super admin manages any tenant's users;
			// the lookup only confirms the user exists.
			if u, err := h.userRepo.FindByID(c.Context(), uint(id), 0); err == nil && u != nil {
				return uint(id), false
			}
			_ = responses.NotFound(c, "user")
			return 0, true
		case middleware.RoleAdmin:
			callerAccount, _ := c.Locals("account_id").(uint)
			if callerAccount != 0 {
				if u, err := h.userRepo.FindByID(c.Context(), uint(id), callerAccount); err == nil && u != nil {
					return uint(id), false
				}
			}
			_ = responses.NotFound(c, "user")
			return 0, true
		}
	}
	_ = responses.Error(c, fiber.StatusForbidden, forbiddenMsg)
	return 0, true
}

// parseTokenExpiry reads token[expires_at]: an RFC 3339 timestamp or a
// date (end of that day, UTC). "" / null means no expiry.
func parseTokenExpiry(raw *string) (*time.Time, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	v := strings.TrimSpace(*raw)
	if v == "" || v == "null" {
		return nil, true, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t, false, nil
	}
	if d, err := time.Parse("2006-01-02", v); err == nil {
		end := d.Add(24*time.Hour - time.Second)
		return &end, false, nil
	}
	return nil, false, errors.New("invalid expires_at: use an ISO 8601 date or timestamp")
}

func tokenCallerID(c *fiber.Ctx) uint {
	id, _ := c.Locals("user_id").(uint)
	return id
}

// ListAccessTokens returns a paginated list of access tokens for the given user.
// GET /api/v1/users/:user_id/tokens (the user, or an admin of their tenant)
func (h *AccessTokenHandler) ListAccessTokens(c *fiber.Ctx) error {
	userID, wrote := h.tokenOwner(c, true, "You can only view your own tokens")
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.tokenService.ListUserTokens(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch access tokens")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	tokens := make([]fiber.Map, len(result.Items))
	for i := range result.Items {
		tokens[i] = accessTokenToJSON(&result.Items[i], tokenCallerID(c))
	}

	return c.JSON(tokens)
}

// ListUserGeneratedTokens lists only the user's personal access tokens.
// GET /api/v1/users/:user_id/user_generated_tokens (the user, or an admin)
func (h *AccessTokenHandler) ListUserGeneratedTokens(c *fiber.Ctx) error {
	userID, wrote := h.tokenOwner(c, true, "You can only view your own tokens")
	if wrote {
		return nil
	}
	params := middleware.GetPagination(c)
	result, err := h.tokenService.ListUserGeneratedTokens(c.Context(), userID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch access tokens")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
	tokens := make([]fiber.Map, len(result.Items))
	for i := range result.Items {
		tokens[i] = accessTokenToJSON(&result.Items[i], tokenCallerID(c))
	}
	return c.JSON(tokens)
}

// GetAccessToken shows one token, by id or token_hint.
// GET /api/v1/users/:user_id/tokens/:id (the user, or an admin)
func (h *AccessTokenHandler) GetAccessToken(c *fiber.Ctx) error {
	userID, wrote := h.tokenOwner(c, true, "You can only view your own tokens")
	if wrote {
		return nil
	}
	token, err := h.tokenService.FindUserToken(c.Context(), userID, c.Params("id"))
	if err != nil {
		return responses.NotFound(c, "access token")
	}
	return c.JSON(accessTokenToJSON(token, tokenCallerID(c)))
}

type createAccessTokenRequest struct {
	Token struct {
		Purpose   string   `json:"purpose"`
		Scopes    []string `json:"scopes"`
		ExpiresAt *string  `json:"expires_at"`
	} `json:"token"`
}

// validateTokenScopes checks PAT scopes are in the Canvas form the auth
// middleware enforces (B085). Returns wrote=true after writing a 400.
func validateTokenScopes(c *fiber.Ctx, scopes []string) bool {
	for _, scope := range scopes {
		if !service.IsValidScope(scope) {
			_ = responses.BadRequest(c, "Invalid scope "+strconv.Quote(scope)+`: use the form url:GET|/api/v1/courses/:course_id`)
			return true
		}
	}
	return false
}

// CreateAccessToken creates a new personal access token.
// POST /api/v1/users/:user_id/tokens (the user only)
// The full token string is only returned at creation time.
//
// Tokens are only ever minted by their owner: Canvas lets a permissioned
// admin create one on a user's behalf (left pending until the user
// activates it), but Paper has no granular admin permission to gate that,
// and a token handed to an admin is an impersonation credential.
func (h *AccessTokenHandler) CreateAccessToken(c *fiber.Ctx) error {
	userID, wrote := h.tokenOwner(c, false, "You can only create tokens for yourself")
	if wrote {
		return nil
	}

	var input createAccessTokenRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	purpose := strings.TrimSpace(input.Token.Purpose)
	if purpose == "" {
		return responses.BadRequest(c, "Token purpose is required")
	}

	if validateTokenScopes(c, input.Token.Scopes) {
		return nil
	}
	expiresAt, _, err := parseTokenExpiry(input.Token.ExpiresAt)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	token, rawToken, err := h.tokenService.CreatePersonalAccessTokenWithExpiry(
		c.Context(),
		userID,
		purpose,
		input.Token.Scopes,
		expiresAt,
	)
	if errors.Is(err, service.ErrAccessTokenExpiryInPast) {
		return responses.BadRequest(c, err.Error())
	}
	if err != nil {
		return responses.InternalError(c, "Could not create access token")
	}

	// Include the full raw token in the response (one-time only)
	result := accessTokenToJSON(token, userID)
	result["token"] = rawToken
	result["visible_token"] = rawToken

	return c.Status(fiber.StatusCreated).JSON(result)
}

type updateAccessTokenRequest struct {
	Token struct {
		Purpose    *string   `json:"purpose"`
		Scopes     *[]string `json:"scopes"`
		ExpiresAt  *string   `json:"expires_at"`
		Regenerate any       `json:"regenerate"`
	} `json:"token"`
}

func tokenTruthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, err := strconv.ParseBool(t)
		return err == nil && b
	case float64:
		return t != 0
	}
	return false
}

// UpdateAccessToken changes a personal access token's purpose, scopes or
// expiry, or regenerates its secret (token[regenerate]=true; the new
// token is returned once, like at creation).
// PUT /api/v1/users/:user_id/tokens/:id (the user only)
func (h *AccessTokenHandler) UpdateAccessToken(c *fiber.Ctx) error {
	userID, wrote := h.tokenOwner(c, false, "You can only change your own tokens")
	if wrote {
		return nil
	}
	token, err := h.tokenService.FindUserToken(c.Context(), userID, c.Params("id"))
	if err != nil {
		return responses.NotFound(c, "access token")
	}
	var input updateAccessTokenRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	update := service.PersonalAccessTokenUpdate{Regenerate: tokenTruthy(input.Token.Regenerate)}
	if input.Token.Purpose != nil {
		p := strings.TrimSpace(*input.Token.Purpose)
		if p == "" {
			return responses.BadRequest(c, "Token purpose cannot be blank")
		}
		update.Purpose = &p
	}
	if input.Token.Scopes != nil {
		if validateTokenScopes(c, *input.Token.Scopes) {
			return nil
		}
		update.Scopes = input.Token.Scopes
	}
	expiresAt, clear, err := parseTokenExpiry(input.Token.ExpiresAt)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	update.ExpiresAt = expiresAt
	update.ClearExpiry = clear

	updated, raw, err := h.tokenService.UpdatePersonalAccessToken(c.Context(), token, update)
	switch {
	case errors.Is(err, service.ErrAccessTokenNotManual),
		errors.Is(err, service.ErrAccessTokenExpiryInPast),
		errors.Is(err, service.ErrAccessTokenRegenerateExpired):
		return responses.BadRequest(c, err.Error())
	case err != nil:
		return responses.InternalError(c, "Could not update access token")
	}
	result := accessTokenToJSON(updated, userID)
	if raw != "" {
		result["token"] = raw
		result["visible_token"] = raw
	}
	return c.JSON(result)
}

// DeleteAccessToken revokes an access token, by id or token_hint.
// DELETE /api/v1/users/:user_id/tokens/:id (the user, or an admin of their
// tenant)
func (h *AccessTokenHandler) DeleteAccessToken(c *fiber.Ctx) error {
	userID, wrote := h.tokenOwner(c, true, "You can only delete your own tokens")
	if wrote {
		return nil
	}

	token, err := h.tokenService.FindUserToken(c.Context(), userID, c.Params("id"))
	if err != nil {
		return responses.NotFound(c, "access token")
	}

	if err := h.tokenService.RevokeToken(c.Context(), token.ID, userID); err != nil {
		return responses.NotFound(c, "access token")
	}

	return c.JSON(fiber.Map{"delete": true})
}
