package handlers

import (
	"fmt"
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

// DownloadDataExport handles GET /api/v1/data_exports/:id/download.
//
// Pre-12.8 the route was promised by FERPAService.ProcessExport (which
// sets DownloadURL to /api/v1/data_exports/:id/download) but never
// existed — every approved export request returned 404 on download.
// This handler streams the ZIP that BuildExportZip assembles, gated by
// the requestor-or-subject-or-admin check inside the service.
func (h *FERPAHandler) DownloadDataExport(c *fiber.Ctx) error {
	exportID, err := c.ParamsInt("id")
	if err != nil || exportID <= 0 {
		return responses.BadRequest(c, "Invalid export request ID")
	}
	callerID, _ := c.Locals("user_id").(uint)
	if callerID == 0 {
		return responses.Unauthorized(c)
	}
	callerIsAdmin, _ := c.Locals("is_admin").(bool)

	// F-006: pass tenant scope through so cross-tenant admin downloads
	// are refused (super_admin bypasses).
	callerAcct, _ := c.Locals("account_id").(uint)
	callerIsSuper, _ := c.Locals("is_super_admin").(bool)
	zipBytes, berr := h.ferpaService.BuildExportZip(c.Context(), uint(exportID), callerID, callerAcct, callerIsAdmin, callerIsSuper)
	if berr != nil {
		switch berr {
		case service.ErrExportForbidden:
			return responses.Forbidden(c, "not authorized to download this export")
		case service.ErrExportNotReady:
			return responses.BadRequest(c, "export is not yet completed")
		case service.ErrExportExpired:
			return responses.Error(c, fiber.StatusGone, "export download link has expired")
		}
		return responses.NotFound(c, "export request")
	}

	filename := fmt.Sprintf("data-export-%d.zip", exportID)
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	return c.Send(zipBytes)
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

// ListPendingDeletionRequests handles GET /api/v1/admin/data_deletion_requests.
//
// F-005: tenant-scoped. The pre-fix path returned every tenant's
// pending requests to any admin (a privacy disclosure that named the
// subject user ID, reason, and request type). Super-admin retains the
// global view.
func (h *FERPAHandler) ListPendingDeletionRequests(c *fiber.Ctx) error {
	params := middleware.GetPagination(c)

	scope := uint(0)
	if isSuper, _ := c.Locals("is_super_admin").(bool); !isSuper {
		scope = callerAccountID(c)
	}

	result, err := h.ferpaService.ListPendingDeletionRequests(c.Context(), scope, params)
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
	reviewerAcct, _ := c.Locals("account_id").(uint)
	isSuper, _ := c.Locals("is_super_admin").(bool)

	// F-005: cross-tenant approval = 404 (existence-leak contract). The
	// pre-fix path let any admin approve any tenant's deletion, which
	// then anonymized the subject user.
	if err := h.ferpaService.ApproveDeletionRequest(c.Context(), uint(requestID), reviewerID, reviewerAcct, isSuper); err != nil {
		if err == service.ErrFERPACrossTenant {
			return responses.NotFound(c, "deletion request")
		}
		return responses.BadRequest(c, err.Error())
	}

	request, err := h.ferpaService.GetDeletionRequest(c.Context(), uint(requestID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch updated request")
	}

	return c.JSON(dataDeletionRequestToJSON(request))
}

// DenyDeletionRequest handles POST /api/v1/admin/data_deletion_requests/:id/deny.
// Mirrors ApproveDeletionRequest's tenant gating: cross-tenant or
// non-super-admin attempts to deny another tenant's deletion request
// return 404 per the existence-leak contract.
func (h *FERPAHandler) DenyDeletionRequest(c *fiber.Ctx) error {
	requestID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid request ID")
	}

	reviewerID, _ := c.Locals("user_id").(uint)
	reviewerAcct, _ := c.Locals("account_id").(uint)
	isSuper, _ := c.Locals("is_super_admin").(bool)

	if err := h.ferpaService.DenyDeletionRequest(c.Context(), uint(requestID), reviewerID, reviewerAcct, isSuper); err != nil {
		if err == service.ErrFERPACrossTenant {
			return responses.NotFound(c, "deletion request")
		}
		return responses.BadRequest(c, err.Error())
	}

	request, err := h.ferpaService.GetDeletionRequest(c.Context(), uint(requestID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch updated request")
	}

	return c.JSON(dataDeletionRequestToJSON(request))
}

// ProcessDeletionRequest handles POST /admin/data_deletion_requests/:id/process.
// EXECUTES an already-approved deletion: anonymizes the subject user row
// and erases dependent-table PII. Kept separate from approval so execution
// is a deliberate, audited step (and reversible up to this point). Mirrors
// the approve handler's cross-tenant existence-leak contract.
func (h *FERPAHandler) ProcessDeletionRequest(c *fiber.Ctx) error {
	requestID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid request ID")
	}

	reviewerAcct, _ := c.Locals("account_id").(uint)
	isSuper, _ := c.Locals("is_super_admin").(bool)

	if err := h.ferpaService.ProcessDeletionScoped(c.Context(), uint(requestID), reviewerAcct, isSuper); err != nil {
		if err == service.ErrFERPACrossTenant {
			return responses.NotFound(c, "deletion request")
		}
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

// ListRetentionPolicies handles GET /api/v1/admin/retention_policies.
//
// F-007: returns the CALLER's tenant's retention policies. The pre-fix
// path hardcoded accountID=1, which (a) leaked tenant-1's policies to
// every admin and (b) hid every other tenant's policies even from
// their own admins. Super-admin sees account 0 — interpreted here as
// "the root account = 1" for backward compat until the hierarchy walk
// lands; super_admin can pass ?account_id= to see another tenant's.
func (h *FERPAHandler) ListRetentionPolicies(c *fiber.Ctx) error {
	accountID := callerAccountID(c)
	if isSuper, _ := c.Locals("is_super_admin").(bool); isSuper {
		if q := c.QueryInt("account_id"); q > 0 {
			accountID = uint(q)
		}
	}

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

// CreateRetentionPolicy handles POST /api/v1/admin/retention_policies.
//
// F-007: creates the policy in the CALLER's tenant. The pre-fix path
// hardcoded accountID=1, which let an admin in tenant 9 silently
// inject policies into tenant 1.
func (h *FERPAHandler) CreateRetentionPolicy(c *fiber.Ctx) error {
	accountID := callerAccountID(c)

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
		Description:     service.SanitizeHTML(input.Description),
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

	// F-007: 404 on cross-tenant — existence-leak contract.
	if assertSameTenant(c, policy.AccountID) {
		return nil
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

	// F-007: 404 on cross-tenant.
	if assertSameTenant(c, policy.AccountID) {
		return nil
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
		policy.Description = service.SanitizeHTML(*input.Description)
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

	// F-007: load first to assert tenant. Delete without the assertion
	// would let any admin in any tenant nuke any policy by guessing IDs.
	policy, err := h.ferpaService.GetRetentionPolicy(c.Context(), uint(policyID))
	if err != nil {
		return responses.NotFound(c, "retention policy")
	}
	if assertSameTenant(c, policy.AccountID) {
		return nil
	}

	if err := h.ferpaService.DeleteRetentionPolicy(c.Context(), uint(policyID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"delete": true})
}
