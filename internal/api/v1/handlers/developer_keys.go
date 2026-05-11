package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type DeveloperKeyHandler struct {
	devKeyService *service.DeveloperKeyService
}

func NewDeveloperKeyHandler(devKeyService *service.DeveloperKeyService) *DeveloperKeyHandler {
	return &DeveloperKeyHandler{devKeyService: devKeyService}
}

// developerKeyToJSON serializes a developer key for API responses.
// The ClientSecret is NEVER included in list/get responses.
func developerKeyToJSON(key *models.DeveloperKey) fiber.Map {
	return fiber.Map{
		"id":             key.ID,
		"name":           key.Name,
		"email":          key.Email,
		"api_key":        key.ClientID,
		"redirect_uri":   key.RedirectURI,
		"redirect_uris":  key.RedirectURIs,
		"icon_url":       key.Icon,
		"notes":          key.Notes,
		"scopes":         key.Scopes,
		"require_scopes": key.RequireScopes,
		"workflow_state": key.WorkflowState,
		"is_lti_key":     key.IsLTIKey,
		"created_at":     key.CreatedAt,
	}
}

// ListDeveloperKeys returns a paginated list of developer keys for an account.
// GET /api/v1/accounts/:account_id/developer_keys
func (h *DeveloperKeyHandler) ListDeveloperKeys(c *fiber.Ctx) error {
	accountID, err := strconv.Atoi(c.Params("account_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.devKeyService.List(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch developer keys")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	keys := make([]fiber.Map, len(result.Items))
	for i, key := range result.Items {
		keys[i] = developerKeyToJSON(&key)
	}

	return c.JSON(keys)
}

// GetDeveloperKey returns a single developer key.
// GET /api/v1/accounts/:account_id/developer_keys/:id
func (h *DeveloperKeyHandler) GetDeveloperKey(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid developer key ID")
	}

	key, err := h.devKeyService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "developer key")
	}

	return c.JSON(developerKeyToJSON(key))
}

type createDeveloperKeyRequest struct {
	DeveloperKey struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		RedirectURI   string `json:"redirect_uri"`
		RedirectURIs  string `json:"redirect_uris"`
		Scopes        string `json:"scopes"`
		RequireScopes *bool  `json:"require_scopes"`
		Notes         string `json:"notes"`
		Icon          string `json:"icon_url"`
		IsLTIKey      *bool  `json:"is_lti_key"`
	} `json:"developer_key"`
}

// CreateDeveloperKey creates a new developer key.
// POST /api/v1/accounts/:account_id/developer_keys
// On create only, the response includes api_key (ClientID) and client_secret (one-time).
func (h *DeveloperKeyHandler) CreateDeveloperKey(c *fiber.Ctx) error {
	accountID, err := strconv.Atoi(c.Params("account_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	var input createDeveloperKeyRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.DeveloperKey.Name == "" {
		return responses.BadRequest(c, "Developer key name is required")
	}

	key := &models.DeveloperKey{
		AccountID:    uint(accountID),
		Name:         input.DeveloperKey.Name,
		Email:        input.DeveloperKey.Email,
		RedirectURI:  input.DeveloperKey.RedirectURI,
		RedirectURIs: input.DeveloperKey.RedirectURIs,
		Scopes:       input.DeveloperKey.Scopes,
		Notes:        input.DeveloperKey.Notes,
		Icon:         input.DeveloperKey.Icon,
	}

	if input.DeveloperKey.RequireScopes != nil {
		key.RequireScopes = *input.DeveloperKey.RequireScopes
	}
	if input.DeveloperKey.IsLTIKey != nil {
		key.IsLTIKey = *input.DeveloperKey.IsLTIKey
	}

	if err := h.devKeyService.Create(c.Context(), key); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// On create, include api_key and the one-time client_secret
	result := developerKeyToJSON(key)
	result["client_secret"] = key.ClientSecret

	return c.Status(fiber.StatusCreated).JSON(result)
}

// UpdateDeveloperKey updates an existing developer key.
// PUT /api/v1/accounts/:account_id/developer_keys/:id
func (h *DeveloperKeyHandler) UpdateDeveloperKey(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid developer key ID")
	}

	key, err := h.devKeyService.GetByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "developer key")
	}

	var input struct {
		DeveloperKey struct {
			Name          *string `json:"name"`
			Email         *string `json:"email"`
			RedirectURI   *string `json:"redirect_uri"`
			RedirectURIs  *string `json:"redirect_uris"`
			Scopes        *string `json:"scopes"`
			RequireScopes *bool   `json:"require_scopes"`
			Notes         *string `json:"notes"`
			Icon          *string `json:"icon_url"`
			WorkflowState *string `json:"workflow_state"`
			IsLTIKey      *bool   `json:"is_lti_key"`
		} `json:"developer_key"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.DeveloperKey.Name != nil {
		key.Name = *input.DeveloperKey.Name
	}
	if input.DeveloperKey.Email != nil {
		key.Email = *input.DeveloperKey.Email
	}
	if input.DeveloperKey.RedirectURI != nil {
		key.RedirectURI = *input.DeveloperKey.RedirectURI
	}
	if input.DeveloperKey.RedirectURIs != nil {
		key.RedirectURIs = *input.DeveloperKey.RedirectURIs
	}
	if input.DeveloperKey.Scopes != nil {
		key.Scopes = *input.DeveloperKey.Scopes
	}
	if input.DeveloperKey.RequireScopes != nil {
		key.RequireScopes = *input.DeveloperKey.RequireScopes
	}
	if input.DeveloperKey.Notes != nil {
		key.Notes = *input.DeveloperKey.Notes
	}
	if input.DeveloperKey.Icon != nil {
		key.Icon = *input.DeveloperKey.Icon
	}
	if input.DeveloperKey.WorkflowState != nil {
		state := *input.DeveloperKey.WorkflowState
		if state != "active" && state != "inactive" {
			return responses.BadRequest(c, "workflow_state must be 'active' or 'inactive'")
		}
		key.WorkflowState = state
	}
	if input.DeveloperKey.IsLTIKey != nil {
		key.IsLTIKey = *input.DeveloperKey.IsLTIKey
	}

	if err := h.devKeyService.Update(c.Context(), key); err != nil {
		return responses.InternalError(c, "Could not update developer key")
	}

	return c.JSON(developerKeyToJSON(key))
}

// DeleteDeveloperKey soft-deletes a developer key.
// DELETE /api/v1/accounts/:account_id/developer_keys/:id
func (h *DeveloperKeyHandler) DeleteDeveloperKey(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid developer key ID")
	}

	if err := h.devKeyService.Delete(c.Context(), uint(id)); err != nil {
		return responses.NotFound(c, "developer key")
	}

	return c.JSON(fiber.Map{"delete": true})
}
