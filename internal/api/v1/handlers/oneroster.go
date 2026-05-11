package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type OneRosterHandler struct {
	onerosterService *service.OneRosterService
}

func NewOneRosterHandler(onerosterService *service.OneRosterService) *OneRosterHandler {
	return &OneRosterHandler{onerosterService: onerosterService}
}

func connectionToJSON(c *models.OneRosterConnection, maskSecret bool) fiber.Map {
	secret := c.ClientSecret
	if maskSecret && len(secret) > 4 {
		secret = "****" + secret[len(secret)-4:]
	} else if maskSecret {
		secret = "****"
	}

	return fiber.Map{
		"id":                 c.ID,
		"account_id":         c.AccountID,
		"name":               c.Name,
		"base_url":           c.BaseURL,
		"client_id":          c.ClientID,
		"client_secret":      secret,
		"token_url":          c.TokenURL,
		"scope":              c.Scope,
		"last_sync_at":       c.LastSyncAt,
		"sync_status":        c.SyncStatus,
		"last_sync_error":    c.LastSyncError,
		"sync_filter":        c.SyncFilter,
		"auto_sync":          c.AutoSync,
		"auto_sync_interval": c.AutoSyncInterval,
		"workflow_state":     c.WorkflowState,
		"created_at":         c.CreatedAt,
		"updated_at":         c.UpdatedAt,
	}
}

func syncLogToJSON(l *models.OneRosterSyncLog) fiber.Map {
	return fiber.Map{
		"id":                   l.ID,
		"connection_id":       l.ConnectionID,
		"sync_type":           l.SyncType,
		"status":              l.Status,
		"orgs_created":        l.OrgsCreated,
		"orgs_updated":        l.OrgsUpdated,
		"users_created":       l.UsersCreated,
		"users_updated":       l.UsersUpdated,
		"classes_created":     l.ClassesCreated,
		"classes_updated":     l.ClassesUpdated,
		"enrollments_created": l.EnrollmentsCreated,
		"enrollments_updated": l.EnrollmentsUpdated,
		"errors":              l.Errors,
		"started_at":          l.StartedAt,
		"completed_at":        l.CompletedAt,
		"error_details":       l.ErrorDetails,
	}
}

// ListConnections handles GET /accounts/:account_id/oneroster_connections
func (h *OneRosterHandler) ListConnections(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.onerosterService.ListConnections(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch OneRoster connections")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	connections := make([]fiber.Map, len(result.Items))
	for i, conn := range result.Items {
		connections[i] = connectionToJSON(&conn, true)
	}

	return c.JSON(connections)
}

// CreateConnection handles POST /accounts/:account_id/oneroster_connections
func (h *OneRosterHandler) CreateConnection(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	var input struct {
		Name             string `json:"name"`
		BaseURL          string `json:"base_url"`
		ClientID         string `json:"client_id"`
		ClientSecret     string `json:"client_secret"`
		TokenURL         string `json:"token_url"`
		Scope            string `json:"scope"`
		SyncFilter       string `json:"sync_filter"`
		AutoSync         bool   `json:"auto_sync"`
		AutoSyncInterval int    `json:"auto_sync_interval"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	conn := &models.OneRosterConnection{
		AccountID:        uint(accountID),
		Name:             input.Name,
		BaseURL:          input.BaseURL,
		ClientID:         input.ClientID,
		ClientSecret:     input.ClientSecret,
		TokenURL:         input.TokenURL,
		Scope:            input.Scope,
		SyncFilter:       input.SyncFilter,
		AutoSync:         input.AutoSync,
		AutoSyncInterval: input.AutoSyncInterval,
	}

	if err := h.onerosterService.CreateConnection(c.Context(), conn); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(connectionToJSON(conn, true))
}

// GetConnection handles GET /accounts/:account_id/oneroster_connections/:id
func (h *OneRosterHandler) GetConnection(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	conn, err := h.onerosterService.GetConnection(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "OneRoster connection")
	}

	return c.JSON(connectionToJSON(conn, true))
}

// UpdateConnection handles PUT /accounts/:account_id/oneroster_connections/:id
func (h *OneRosterHandler) UpdateConnection(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	conn, err := h.onerosterService.GetConnection(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "OneRoster connection")
	}

	var input struct {
		Name             *string `json:"name"`
		BaseURL          *string `json:"base_url"`
		ClientID         *string `json:"client_id"`
		ClientSecret     *string `json:"client_secret"`
		TokenURL         *string `json:"token_url"`
		Scope            *string `json:"scope"`
		SyncFilter       *string `json:"sync_filter"`
		AutoSync         *bool   `json:"auto_sync"`
		AutoSyncInterval *int    `json:"auto_sync_interval"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Name != nil {
		conn.Name = *input.Name
	}
	if input.BaseURL != nil {
		conn.BaseURL = *input.BaseURL
	}
	if input.ClientID != nil {
		conn.ClientID = *input.ClientID
	}
	if input.ClientSecret != nil && *input.ClientSecret != "" {
		conn.ClientSecret = *input.ClientSecret
	}
	if input.TokenURL != nil {
		conn.TokenURL = *input.TokenURL
	}
	if input.Scope != nil {
		conn.Scope = *input.Scope
	}
	if input.SyncFilter != nil {
		conn.SyncFilter = *input.SyncFilter
	}
	if input.AutoSync != nil {
		conn.AutoSync = *input.AutoSync
	}
	if input.AutoSyncInterval != nil {
		conn.AutoSyncInterval = *input.AutoSyncInterval
	}

	if err := h.onerosterService.UpdateConnection(c.Context(), conn); err != nil {
		return responses.InternalError(c, "Could not update connection")
	}

	return c.JSON(connectionToJSON(conn, true))
}

// DeleteConnection handles DELETE /accounts/:account_id/oneroster_connections/:id
func (h *OneRosterHandler) DeleteConnection(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	if err := h.onerosterService.DeleteConnection(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not delete connection")
	}

	return c.JSON(fiber.Map{"message": "Connection deleted"})
}

// TestConnection handles POST /accounts/:account_id/oneroster_connections/:id/test
func (h *OneRosterHandler) TestConnection(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	success, message, err := h.onerosterService.TestConnection(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"success": success,
		"message": message,
	})
}

// SyncFull handles POST /accounts/:account_id/oneroster_connections/:id/sync
func (h *OneRosterHandler) SyncFull(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	syncLog, err := h.onerosterService.SyncFull(c.Context(), uint(id))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(syncLogToJSON(syncLog))
}

// SyncIncremental handles POST /accounts/:account_id/oneroster_connections/:id/sync_incremental
func (h *OneRosterHandler) SyncIncremental(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	syncLog, err := h.onerosterService.SyncIncremental(c.Context(), uint(id))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(syncLogToJSON(syncLog))
}

// GetSyncLogs handles GET /accounts/:account_id/oneroster_connections/:id/sync_logs
func (h *OneRosterHandler) GetSyncLogs(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.onerosterService.GetSyncLogs(c.Context(), uint(id), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch sync logs")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	logs := make([]fiber.Map, len(result.Items))
	for i, log := range result.Items {
		logs[i] = syncLogToJSON(&log)
	}

	return c.JSON(logs)
}

// GetSyncStatus handles GET /accounts/:account_id/oneroster_connections/:id/sync_status
func (h *OneRosterHandler) GetSyncStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid connection ID")
	}

	conn, latestLog, err := h.onerosterService.GetSyncStatus(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "OneRoster connection")
	}

	result := fiber.Map{
		"sync_status":    conn.SyncStatus,
		"last_sync_at":   conn.LastSyncAt,
		"last_sync_error": conn.LastSyncError,
	}

	if latestLog != nil {
		result["latest_sync_log"] = syncLogToJSON(latestLog)
	}

	return c.JSON(result)
}
