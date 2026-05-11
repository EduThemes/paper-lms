package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AccessTokenHandler struct {
	tokenService *service.AccessTokenService
}

func NewAccessTokenHandler(tokenService *service.AccessTokenService) *AccessTokenHandler {
	return &AccessTokenHandler{tokenService: tokenService}
}

// accessTokenToJSON serializes an access token for API responses.
// The raw token value and refresh token are NEVER included here.
func accessTokenToJSON(token *models.AccessToken) fiber.Map {
	return fiber.Map{
		"id":           token.ID,
		"purpose":      token.Purpose,
		"token_hint":   token.TokenHint,
		"scopes":       token.Scopes,
		"expires_at":   token.ExpiresAt,
		"last_used_at": token.LastUsedAt,
		"created_at":   token.CreatedAt,
	}
}

// ListAccessTokens returns a paginated list of access tokens for the given user.
// GET /api/v1/users/:user_id/tokens
func (h *AccessTokenHandler) ListAccessTokens(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	// Verify the requesting user matches the target user
	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}
	if currentUserID != uint(userID) {
		return responses.Error(c, fiber.StatusForbidden, "You can only view your own tokens")
	}

	params := middleware.GetPagination(c)

	result, err := h.tokenService.ListUserTokens(c.Context(), uint(userID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch access tokens")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	tokens := make([]fiber.Map, len(result.Items))
	for i, token := range result.Items {
		tokens[i] = accessTokenToJSON(&token)
	}

	return c.JSON(tokens)
}

type createAccessTokenRequest struct {
	Token struct {
		Purpose string   `json:"purpose"`
		Scopes  []string `json:"scopes"`
	} `json:"token"`
}

// CreateAccessToken creates a new personal access token.
// POST /api/v1/users/:user_id/tokens
// The full token string is only returned at creation time.
func (h *AccessTokenHandler) CreateAccessToken(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	// Verify the requesting user matches the target user
	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}
	if currentUserID != uint(userID) {
		return responses.Error(c, fiber.StatusForbidden, "You can only create tokens for yourself")
	}

	var input createAccessTokenRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	purpose := strings.TrimSpace(input.Token.Purpose)
	if purpose == "" {
		return responses.BadRequest(c, "Token purpose is required")
	}

	token, rawToken, err := h.tokenService.CreatePersonalAccessToken(
		c.Context(),
		uint(userID),
		purpose,
		input.Token.Scopes,
	)
	if err != nil {
		return responses.InternalError(c, "Could not create access token")
	}

	// Include the full raw token in the response (one-time only)
	result := accessTokenToJSON(token)
	result["token"] = rawToken

	return c.Status(fiber.StatusCreated).JSON(result)
}

// DeleteAccessToken revokes a personal access token.
// DELETE /api/v1/users/:user_id/tokens/:id
func (h *AccessTokenHandler) DeleteAccessToken(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	// Verify the requesting user matches the target user
	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}
	if currentUserID != uint(userID) {
		return responses.Error(c, fiber.StatusForbidden, "You can only delete your own tokens")
	}

	tokenID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid token ID")
	}

	if err := h.tokenService.RevokeToken(c.Context(), uint(tokenID), uint(userID)); err != nil {
		if err.Error() == "not authorized to revoke this token" {
			return responses.Error(c, fiber.StatusForbidden, "Not authorized to revoke this token")
		}
		return responses.NotFound(c, "access token")
	}

	return c.JSON(fiber.Map{"delete": true})
}
