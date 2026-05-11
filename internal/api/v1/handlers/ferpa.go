package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// FERPAHandler handles HTTP requests for FERPA compliance endpoints.
type FERPAHandler struct {
	ferpaService *service.FERPAService
}

// NewFERPAHandler creates a new FERPAHandler.
func NewFERPAHandler(ferpaService *service.FERPAService) *FERPAHandler {
	return &FERPAHandler{ferpaService: ferpaService}
}

func dataExportRequestToJSON(req *models.DataExportRequest) fiber.Map {
	return fiber.Map{
		"id":              req.ID,
		"requested_by_id": req.RequestedByID,
		"user_id":         req.UserID,
		"export_format":   req.ExportFormat,
		"data_scope":      req.DataScope,
		"status":          req.Status,
		"download_url":    req.DownloadURL,
		"expires_at":      req.ExpiresAt,
		"completed_at":    req.CompletedAt,
		"file_size_bytes": req.FileSizeBytes,
		"created_at":      req.CreatedAt,
		"updated_at":      req.UpdatedAt,
	}
}

func dataDeletionRequestToJSON(req *models.DataDeletionRequest) fiber.Map {
	return fiber.Map{
		"id":              req.ID,
		"requested_by_id": req.RequestedByID,
		"user_id":         req.UserID,
		"request_type":    req.RequestType,
		"data_scope":      req.DataScope,
		"reason":          req.Reason,
		"status":          req.Status,
		"reviewed_by_id":  req.ReviewedByID,
		"reviewed_at":     req.ReviewedAt,
		"completed_at":    req.CompletedAt,
		"deletion_log":    req.DeletionLog,
		"created_at":      req.CreatedAt,
		"updated_at":      req.UpdatedAt,
	}
}

func piiAccessLogToJSON(log *models.PIIAccessLog) fiber.Map {
	return fiber.Map{
		"id":            log.ID,
		"accessor_id":   log.AccessorID,
		"student_id":    log.StudentID,
		"access_type":   log.AccessType,
		"data_field":    log.DataField,
		"resource":      log.Resource,
		"resource_id":   log.ResourceID,
		"ip_address":    log.IPAddress,
		"user_agent":    log.UserAgent,
		"justification": log.Justification,
		"created_at":    log.CreatedAt,
	}
}

func retentionPolicyToJSON(policy *models.DataRetentionPolicy) fiber.Map {
	return fiber.Map{
		"id":               policy.ID,
		"account_id":       policy.AccountID,
		"data_category":    policy.DataCategory,
		"retention_period": policy.RetentionPeriod,
		"retention_action": policy.RetentionAction,
		"auto_apply":       policy.AutoApply,
		"description":      policy.Description,
		"created_at":       policy.CreatedAt,
		"updated_at":       policy.UpdatedAt,
	}
}

// CreateExportRequest handles POST /api/v1/users/:user_id/data_export
func (h *FERPAHandler) CreateExportRequest(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		ExportFormat string `json:"export_format"`
		DataScope    string `json:"data_scope"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	requestedByID, _ := c.Locals("user_id").(uint)

	request, err := h.ferpaService.CreateExportRequest(c.Context(), requestedByID, uint(userID), input.ExportFormat, input.DataScope)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(dataExportRequestToJSON(request))
}

// GetExportRequest handles GET /api/v1/users/:user_id/data_export/:id
func (h *FERPAHandler) GetExportRequest(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	exportID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid export request ID")
	}

	request, err := h.ferpaService.GetExportRequest(c.Context(), uint(exportID))
	if err != nil {
		return responses.NotFound(c, "export request")
	}

	// Verify the export belongs to the URL's user_id to prevent IDOR
	if request.UserID != uint(userID) {
		return responses.NotFound(c, "export request")
	}

	return c.JSON(dataExportRequestToJSON(request))
}

// CreateDeletionRequest handles POST /api/v1/users/:user_id/data_deletion
func (h *FERPAHandler) CreateDeletionRequest(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		RequestType string `json:"request_type"`
		DataScope   string `json:"data_scope"`
		Reason      string `json:"reason"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	requestedByID, _ := c.Locals("user_id").(uint)

	request, err := h.ferpaService.CreateDeletionRequest(c.Context(), requestedByID, uint(userID), input.RequestType, input.DataScope, input.Reason)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(dataDeletionRequestToJSON(request))
}

// ListPendingDeletionRequests handles GET /api/v1/admin/data_deletion_requests
func (h *FERPAHandler) ListPendingDeletionRequests(c *fiber.Ctx) error {
	params := middleware.GetPagination(c)

	result, err := h.ferpaService.ListPendingDeletionRequests(c.Context(), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch deletion requests")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	requests := make([]fiber.Map, len(result.Items))
	for i, req := range result.Items {
		requests[i] = dataDeletionRequestToJSON(&req)
	}

	return c.JSON(requests)
}

// ApproveDeletionRequest handles PUT /api/v1/deletion_requests/:id/approve
func (h *FERPAHandler) ApproveDeletionRequest(c *fiber.Ctx) error {
	requestID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid request ID")
	}

	reviewerID, _ := c.Locals("user_id").(uint)

	if err := h.ferpaService.ApproveDeletionRequest(c.Context(), uint(requestID), reviewerID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	request, err := h.ferpaService.GetDeletionRequest(c.Context(), uint(requestID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch updated request")
	}

	return c.JSON(dataDeletionRequestToJSON(request))
}

// GetPIIAccessLog handles GET /api/v1/users/:user_id/pii_access_log
func (h *FERPAHandler) GetPIIAccessLog(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("user_id"))
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.ferpaService.ListPIIAccessLogs(c.Context(), uint(userID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch PII access logs")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	logs := make([]fiber.Map, len(result.Items))
	for i, log := range result.Items {
		logs[i] = piiAccessLogToJSON(&log)
	}

	return c.JSON(logs)
}

// ListRetentionPolicies handles GET /api/v1/admin/retention_policies
func (h *FERPAHandler) ListRetentionPolicies(c *fiber.Ctx) error {
	// Default to account 1 (single-tenant admin route)
	accountID := uint(1)

	params := middleware.GetPagination(c)

	result, err := h.ferpaService.ListRetentionPolicies(c.Context(), accountID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch retention policies")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	policies := make([]fiber.Map, len(result.Items))
	for i, policy := range result.Items {
		policies[i] = retentionPolicyToJSON(&policy)
	}

	return c.JSON(policies)
}

// CreateRetentionPolicy handles POST /api/v1/admin/retention_policies
func (h *FERPAHandler) CreateRetentionPolicy(c *fiber.Ctx) error {
	// Default to account 1 (single-tenant admin route)
	accountID := uint(1)

	var input struct {
		DataCategory    string `json:"data_category"`
		RetentionPeriod int    `json:"retention_period"`
		RetentionAction string `json:"retention_action"`
		AutoApply       bool   `json:"auto_apply"`
		Description     string `json:"description"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	policy := &models.DataRetentionPolicy{
		AccountID:       accountID,
		DataCategory:    input.DataCategory,
		RetentionPeriod: input.RetentionPeriod,
		RetentionAction: input.RetentionAction,
		AutoApply:       input.AutoApply,
		Description:     input.Description,
	}

	if err := h.ferpaService.CreateRetentionPolicy(c.Context(), policy); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(retentionPolicyToJSON(policy))
}

// GetRetentionPolicy handles GET /api/v1/accounts/:account_id/retention_policies/:id
func (h *FERPAHandler) GetRetentionPolicy(c *fiber.Ctx) error {
	policyID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid policy ID")
	}

	policy, err := h.ferpaService.GetRetentionPolicy(c.Context(), uint(policyID))
	if err != nil {
		return responses.NotFound(c, "retention policy")
	}

	return c.JSON(retentionPolicyToJSON(policy))
}

// UpdateRetentionPolicy handles PUT /api/v1/accounts/:account_id/retention_policies/:id
func (h *FERPAHandler) UpdateRetentionPolicy(c *fiber.Ctx) error {
	policyID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid policy ID")
	}

	policy, err := h.ferpaService.GetRetentionPolicy(c.Context(), uint(policyID))
	if err != nil {
		return responses.NotFound(c, "retention policy")
	}

	var input struct {
		DataCategory    *string `json:"data_category"`
		RetentionPeriod *int    `json:"retention_period"`
		RetentionAction *string `json:"retention_action"`
		AutoApply       *bool   `json:"auto_apply"`
		Description     *string `json:"description"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.DataCategory != nil {
		policy.DataCategory = *input.DataCategory
	}
	if input.RetentionPeriod != nil {
		policy.RetentionPeriod = *input.RetentionPeriod
	}
	if input.RetentionAction != nil {
		policy.RetentionAction = *input.RetentionAction
	}
	if input.AutoApply != nil {
		policy.AutoApply = *input.AutoApply
	}
	if input.Description != nil {
		policy.Description = *input.Description
	}

	if err := h.ferpaService.UpdateRetentionPolicy(c.Context(), policy); err != nil {
		return responses.InternalError(c, "Could not update retention policy")
	}

	return c.JSON(retentionPolicyToJSON(policy))
}

// DeleteRetentionPolicy handles DELETE /api/v1/accounts/:account_id/retention_policies/:id
func (h *FERPAHandler) DeleteRetentionPolicy(c *fiber.Ctx) error {
	policyID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid policy ID")
	}

	if err := h.ferpaService.DeleteRetentionPolicy(c.Context(), uint(policyID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"delete": true})
}
