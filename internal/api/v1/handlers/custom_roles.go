package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// CustomRoleHandler handles HTTP requests for custom roles and permissions.
type CustomRoleHandler struct {
	customRoleService *service.CustomRoleService
}

// NewCustomRoleHandler creates a new CustomRoleHandler.
func NewCustomRoleHandler(customRoleService *service.CustomRoleService) *CustomRoleHandler {
	return &CustomRoleHandler{customRoleService: customRoleService}
}

func customRoleToJSON(role *models.CustomRole) fiber.Map {
	return fiber.Map{
		"id":                 role.ID,
		"account_id":        role.AccountID,
		"name":              role.Name,
		"base_role_type":    role.BaseRoleType,
		"label":             role.Label,
		"workflow_state":    role.WorkflowState,
		"permissions":       role.Permissions,
		"created_by_user_id": role.CreatedByUserID,
		"created_at":        role.CreatedAt,
		"updated_at":        role.UpdatedAt,
	}
}

func roleOverrideToJSON(o *models.RoleOverride) fiber.Map {
	return fiber.Map{
		"id":           o.ID,
		"account_id":  o.AccountID,
		"role_id":     o.RoleID,
		"permission":  o.Permission,
		"enabled":     o.Enabled,
		"locked":      o.Locked,
		"context_type": o.ContextType,
		"context_id":  o.ContextID,
		"created_at":  o.CreatedAt,
		"updated_at":  o.UpdatedAt,
	}
}

// ListRoles handles GET /accounts/:account_id/roles
func (h *CustomRoleHandler) ListRoles(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.customRoleService.ListRoles(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch roles")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	roles := make([]fiber.Map, len(result.Items))
	for i, r := range result.Items {
		roles[i] = customRoleToJSON(&r)
	}

	return c.JSON(roles)
}

// CreateRole handles POST /accounts/:account_id/roles
func (h *CustomRoleHandler) CreateRole(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Role struct {
			Name         string `json:"name"`
			BaseRoleType string `json:"base_role_type"`
			Label        string `json:"label"`
			Permissions  string `json:"permissions"`
		} `json:"role"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	role := &models.CustomRole{
		AccountID:       uint(accountID),
		Name:            input.Role.Name,
		BaseRoleType:    input.Role.BaseRoleType,
		Label:           input.Role.Label,
		Permissions:     input.Role.Permissions,
		CreatedByUserID: userID,
	}

	if err := h.customRoleService.CreateRole(c.Context(), role); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(customRoleToJSON(role))
}

// GetRole handles GET /accounts/:account_id/roles/:id
func (h *CustomRoleHandler) GetRole(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid role ID")
	}

	role, err := h.customRoleService.GetRole(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "role")
	}

	result := customRoleToJSON(role)

	// Include permission definitions for the UI
	result["permission_definitions"] = models.AllPermissions()

	return c.JSON(result)
}

// UpdateRole handles PUT /accounts/:account_id/roles/:id
func (h *CustomRoleHandler) UpdateRole(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid role ID")
	}

	existing, err := h.customRoleService.GetRole(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "role")
	}

	var input struct {
		Role struct {
			Name         string  `json:"name"`
			BaseRoleType string  `json:"base_role_type"`
			Label        string  `json:"label"`
			Permissions  string  `json:"permissions"`
			WorkflowState string `json:"workflow_state"`
		} `json:"role"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Role.Name != "" {
		existing.Name = input.Role.Name
	}
	if input.Role.BaseRoleType != "" {
		existing.BaseRoleType = input.Role.BaseRoleType
	}
	if input.Role.Label != "" {
		existing.Label = input.Role.Label
	}
	if input.Role.Permissions != "" {
		existing.Permissions = input.Role.Permissions
	}
	if input.Role.WorkflowState != "" {
		existing.WorkflowState = input.Role.WorkflowState
	}

	if err := h.customRoleService.UpdateRole(c.Context(), existing); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(customRoleToJSON(existing))
}

// DeleteRole handles DELETE /accounts/:account_id/roles/:id
func (h *CustomRoleHandler) DeleteRole(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid role ID")
	}

	if err := h.customRoleService.DeleteRole(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete role")
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}

// CloneRole handles POST /accounts/:account_id/roles/:id/clone
func (h *CustomRoleHandler) CloneRole(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid role ID")
	}

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Name string `json:"name"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	clone, err := h.customRoleService.CloneRole(c.Context(), uint(id), input.Name, userID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(customRoleToJSON(clone))
}

// GetPresets handles GET /accounts/:account_id/roles/presets
func (h *CustomRoleHandler) GetPresets(c *fiber.Ctx) error {
	presets := h.customRoleService.GetPermissionPresets()

	result := make([]fiber.Map, len(presets))
	for i, p := range presets {
		result[i] = fiber.Map{
			"name":        p.Name,
			"label":       p.Label,
			"description": p.Description,
			"permissions": p.Permissions,
		}
	}

	return c.JSON(result)
}

// ListOverrides handles GET /accounts/:account_id/roles/:id/overrides
func (h *CustomRoleHandler) ListOverrides(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid role ID")
	}

	overrides, err := h.customRoleService.GetRoleOverrides(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, "Could not fetch overrides")
	}

	result := make([]fiber.Map, len(overrides))
	for i, o := range overrides {
		result[i] = roleOverrideToJSON(&o)
	}

	return c.JSON(result)
}

// BulkSetOverrides handles PUT /accounts/:account_id/roles/:id/overrides
func (h *CustomRoleHandler) BulkSetOverrides(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid role ID")
	}

	var input struct {
		Overrides []struct {
			Permission  string `json:"permission"`
			Enabled     bool   `json:"enabled"`
			Locked      bool   `json:"locked"`
			ContextType string `json:"context_type"`
			ContextID   uint   `json:"context_id"`
		} `json:"overrides"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	overrides := make([]models.RoleOverride, len(input.Overrides))
	for i, o := range input.Overrides {
		overrides[i] = models.RoleOverride{
			Permission:  o.Permission,
			Enabled:     o.Enabled,
			Locked:      o.Locked,
			ContextType: o.ContextType,
			ContextID:   o.ContextID,
		}
	}

	if err := h.customRoleService.BulkSetOverrides(c.Context(), uint(id), overrides); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"status": "updated", "count": len(overrides)})
}

// GetCoursePermissions handles GET /courses/:course_id/permissions
// Returns the current user's effective permissions for a given course.
func (h *CustomRoleHandler) GetCoursePermissions(c *fiber.Ctx) error {
	courseIDStr := c.Params("course_id")
	if courseIDStr == "" {
		courseIDStr = c.Params("id")
	}

	courseID64, err := strconv.ParseUint(courseIDStr, 10, 64)
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	courseID := uint(courseID64)

	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	perms, err := h.customRoleService.GetEffectivePermissions(c.Context(), userID, courseID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Build response with permission details
	allPerms := models.AllPermissions()
	permResults := make([]fiber.Map, len(allPerms))
	for i, p := range allPerms {
		enabled := perms[p.Name]
		permResults[i] = fiber.Map{
			"name":        p.Name,
			"label":       p.Label,
			"description": p.Description,
			"category":    p.Category,
			"enabled":     enabled,
		}
	}

	return c.JSON(fiber.Map{
		"course_id":   courseID,
		"user_id":     userID,
		"permissions": permResults,
	})
}
