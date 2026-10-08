package handlers

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

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
		"scopes":         service.ParseScopes(key.Scopes),
		"require_scopes": key.RequireScopes,
		"workflow_state": key.WorkflowState,
		"is_lti_key":     key.IsLTIKey,
		"created_at":     key.CreatedAt,
	}
}

// resolveDeveloperKeyAccount resolves the URL :account_id to the caller's
// tenant. Developer keys are tenant-owned, so the only account a caller may
// address is their own: "self" (the Canvas alias) or their own numeric id.
// Any other id is a cross-tenant probe and gets the existence-leak 404.
//
// B075: List/Create previously trusted the URL :account_id verbatim, so an
// admin of tenant 7 could list (and create keys in) tenant 3.
//
// Returns (accountID, wrote); wrote==true means a 4xx was already written
// and the caller must `return nil`.
func resolveDeveloperKeyAccount(c *fiber.Ctx) (uint, bool) {
	caller := callerAccountID(c)
	raw := c.Params("account_id")
	if raw == "self" {
		return caller, false
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		_ = responses.BadRequest(c, "Invalid account ID")
		return 0, true
	}
	if uint(id) != caller {
		_ = responses.NotFound(c, "account")
		return 0, true
	}
	return caller, false
}

// flexStringList accepts either a JSON string or a JSON array of strings.
// The Developer Keys UI and Canvas API clients send scopes and
// redirect_uris as arrays; older callers send a single string (B182).
type flexStringList struct {
	Set    bool
	IsList bool
	Str    string
	List   []string
}

func (f *flexStringList) UnmarshalJSON(b []byte) error {
	f.Set = true
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	if len(b) > 0 && b[0] == '[' {
		f.IsList = true
		return json.Unmarshal(b, &f.List)
	}
	return json.Unmarshal(b, &f.Str)
}

// values returns the input as a trimmed, non-empty list. A plain string
// is split on newlines; a JSON-array-shaped string (the scopes storage
// format) is parsed as one.
func (f *flexStringList) values() []string {
	var raw []string
	switch {
	case f.IsList:
		raw = f.List
	case strings.HasPrefix(strings.TrimSpace(f.Str), "["):
		raw = service.ParseScopes(f.Str)
	default:
		raw = strings.Split(f.Str, "\n")
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// redirectURIsValue is the newline-separated storage form of redirect_uris.
func (f *flexStringList) redirectURIsValue() string {
	return strings.Join(f.values(), "\n")
}

// scopesValue is the JSON-array storage form of scopes.
func (f *flexStringList) scopesValue() string {
	return service.ScopesToJSON(f.values())
}

// invalidScope returns the first scope that is not an enforceable Canvas
// url: scope, or "" when all are valid (B085).
func (f *flexStringList) invalidScope() string {
	for _, s := range f.values() {
		if !service.IsValidScope(s) {
			return s
		}
	}
	return ""
}

// ListDeveloperKeys returns a paginated list of developer keys for an account.
// GET /api/v1/accounts/:account_id/developer_keys
func (h *DeveloperKeyHandler) ListDeveloperKeys(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.devKeyService.List(c.Context(), accountID, params)
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
	if _, wrote := resolveDeveloperKeyAccount(c); wrote {
		return nil
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid developer key ID")
	}

	key, err := h.devKeyService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "developer key")
	}

	return c.JSON(developerKeyToJSON(key))
}

type createDeveloperKeyRequest struct {
	DeveloperKey struct {
		Name          string         `json:"name"`
		Email         string         `json:"email"`
		RedirectURI   string         `json:"redirect_uri"`
		RedirectURIs  flexStringList `json:"redirect_uris"`
		Scopes        flexStringList `json:"scopes"`
		RequireScopes *bool          `json:"require_scopes"`
		Notes         string         `json:"notes"`
		Icon          string         `json:"icon_url"`
		IsLTIKey      *bool          `json:"is_lti_key"`
	} `json:"developer_key"`
}

// CreateDeveloperKey creates a new developer key.
// POST /api/v1/accounts/:account_id/developer_keys
// On create only, the response includes api_key (ClientID) and client_secret (one-time).
func (h *DeveloperKeyHandler) CreateDeveloperKey(c *fiber.Ctx) error {
	accountID, wrote := resolveDeveloperKeyAccount(c)
	if wrote {
		return nil
	}

	var input createDeveloperKeyRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.DeveloperKey.Name == "" {
		return responses.BadRequest(c, "Developer key name is required")
	}
	if bad := input.DeveloperKey.Scopes.invalidScope(); bad != "" {
		return responses.BadRequest(c, "Invalid scope "+strconv.Quote(bad)+": use the form url:GET|/api/v1/courses/:course_id")
	}

	key := &models.DeveloperKey{
		AccountID:    accountID,
		Name:         input.DeveloperKey.Name,
		Email:        input.DeveloperKey.Email,
		RedirectURI:  input.DeveloperKey.RedirectURI,
		RedirectURIs: input.DeveloperKey.RedirectURIs.redirectURIsValue(),
		Scopes:       input.DeveloperKey.Scopes.scopesValue(),
		Notes:        input.DeveloperKey.Notes,
		Icon:         input.DeveloperKey.Icon,
	}

	if input.DeveloperKey.RequireScopes != nil {
		key.RequireScopes = *input.DeveloperKey.RequireScopes
	}
	if input.DeveloperKey.IsLTIKey != nil {
		key.IsLTIKey = *input.DeveloperKey.IsLTIKey
	}

	rawSecret, err := h.devKeyService.Create(c.Context(), key)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// On create, include api_key and the one-time client_secret. Only a
	// hash of the secret is persisted (B191), so this is the single
	// moment the plaintext exists.
	result := developerKeyToJSON(key)
	result["client_secret"] = rawSecret

	return c.Status(fiber.StatusCreated).JSON(result)
}

// UpdateDeveloperKey updates an existing developer key.
// PUT /api/v1/accounts/:account_id/developer_keys/:id
func (h *DeveloperKeyHandler) UpdateDeveloperKey(c *fiber.Ctx) error {
	if _, wrote := resolveDeveloperKeyAccount(c); wrote {
		return nil
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid developer key ID")
	}

	key, err := h.devKeyService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "developer key")
	}

	var input struct {
		DeveloperKey struct {
			Name          *string        `json:"name"`
			Email         *string        `json:"email"`
			RedirectURI   *string        `json:"redirect_uri"`
			RedirectURIs  flexStringList `json:"redirect_uris"`
			Scopes        flexStringList `json:"scopes"`
			RequireScopes *bool          `json:"require_scopes"`
			Notes         *string        `json:"notes"`
			Icon          *string        `json:"icon_url"`
			WorkflowState *string        `json:"workflow_state"`
			IsLTIKey      *bool          `json:"is_lti_key"`
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
	if input.DeveloperKey.RedirectURIs.Set {
		key.RedirectURIs = input.DeveloperKey.RedirectURIs.redirectURIsValue()
	}
	if input.DeveloperKey.Scopes.Set {
		if bad := input.DeveloperKey.Scopes.invalidScope(); bad != "" {
			return responses.BadRequest(c, "Invalid scope "+strconv.Quote(bad)+": use the form url:GET|/api/v1/courses/:course_id")
		}
		key.Scopes = input.DeveloperKey.Scopes.scopesValue()
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

	if err := h.devKeyService.Update(c.Context(), key, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not update developer key")
	}

	return c.JSON(developerKeyToJSON(key))
}

// DeleteDeveloperKey soft-deletes a developer key.
// DELETE /api/v1/accounts/:account_id/developer_keys/:id
func (h *DeveloperKeyHandler) DeleteDeveloperKey(c *fiber.Ctx) error {
	if _, wrote := resolveDeveloperKeyAccount(c); wrote {
		return nil
	}
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid developer key ID")
	}

	if err := h.devKeyService.Delete(c.Context(), uint(id), callerAccountID(c)); err != nil {
		return responses.NotFound(c, "developer key")
	}

	return c.JSON(fiber.Map{"delete": true})
}
