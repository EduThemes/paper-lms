package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type AccountHandler struct {
	accountRepo repository.AccountRepository
}

func NewAccountHandler(accountRepo repository.AccountRepository) *AccountHandler {
	return &AccountHandler{accountRepo: accountRepo}
}

func accountToJSON(a *models.Account) fiber.Map {
	return fiber.Map{
		"id":                 a.ID,
		"name":               a.Name,
		"parent_account_id":  a.ParentAccountID,
		"root_account_id":    a.RootAccountID,
		"workflow_state":     a.WorkflowState,
		"max_upload_size_mb": a.MaxUploadSizeMB,
		"tenant_mode":        string(a.TenantMode),
	}
}

// ListAccounts returns the caller's own tenant (and child accounts when
// the hierarchy lands). A super_admin sees every account in the
// deployment; an account-admin sees only their own row.
//
// SECURITY (F-001): the pre-fix path returned every account in the DB
// to any admin, which let a tenant-A admin enumerate every tenant in
// the deployment.
func (h *AccountHandler) ListAccounts(c *fiber.Ctx) error {
	params := middleware.GetPagination(c)

	isSuper, _ := c.Locals("is_super_admin").(bool)
	if isSuper {
		result, err := h.accountRepo.List(c.Context(), params)
		if err != nil {
			return responses.InternalError(c, "Could not fetch accounts")
		}
		responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
		accounts := make([]fiber.Map, len(result.Items))
		for i := range result.Items {
			accounts[i] = accountToJSON(&result.Items[i])
		}
		return c.JSON(accounts)
	}

	// Tenant-admin: scope to their own account row. The hierarchy walk
	// (parent/child) is a Phase-14 follow-up; for now return the
	// caller's account only — that's strictly less data than before,
	// and the audit's "multi-tenancy in disguise" finding is closed.
	account, err := h.accountRepo.FindByID(c.Context(), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch account")
	}
	responses.SetPaginationHeaders(c, 1, 1, 1)
	return c.JSON([]fiber.Map{accountToJSON(account)})
}

func (h *AccountHandler) GetAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	// SECURITY (F-001): require :id == caller's tenant. Super-admin
	// bypasses (treated as global operator). 404 on mismatch leaks
	// nothing about the existence of other tenants.
	if assertSameTenant(c, uint(id)) {
		return nil
	}

	account, err := h.accountRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "account")
	}

	return c.JSON(accountToJSON(account))
}

// UpdateAccount lets an admin edit account-level settings.
// Exposes name, max_upload_size_mb, and tenant_mode.
func (h *AccountHandler) UpdateAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	// SECURITY (F-001): see GetAccount.
	if assertSameTenant(c, uint(id)) {
		return nil
	}

	account, err := h.accountRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "account")
	}

	var input struct {
		Name       *string `json:"name"`
		TenantMode *string `json:"tenant_mode"`
		Settings   *struct {
			MaxUploadSizeMB *uint `json:"max_upload_size_mb"`
		} `json:"settings"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Name != nil && *input.Name != "" {
		account.Name = *input.Name
	}
	// tenant_mode drives every gamification + privacy default; the
	// leaderboard RenderPolicy reads it (RenderPolicyFor) to decide
	// what students see. Locked to the six gamification_audience
	// enum values; an unknown string is rejected with 400 rather than
	// silently coerced.
	if input.TenantMode != nil {
		switch models.GamificationAudience(*input.TenantMode) {
		case models.AudienceK5, models.AudienceM68, models.AudienceH912,
			models.AudienceHigherEd, models.AudienceCorp, models.AudiencePro:
			account.TenantMode = models.GamificationAudience(*input.TenantMode)
		default:
			return responses.BadRequest(c, "invalid tenant_mode; must be one of k5, m68, h912, higher_ed, corp, pro")
		}
	}
	if input.Settings != nil && input.Settings.MaxUploadSizeMB != nil {
		v := *input.Settings.MaxUploadSizeMB
		// Sanity bounds: 1 MB minimum, 5120 MB (5 GB) maximum to match the
		// Fiber-level safety net. Admins shouldn't be able to set values that
		// would always fail at the framework level.
		if v < 1 {
			v = 1
		}
		if v > 5120 {
			v = 5120
		}
		account.MaxUploadSizeMB = v
	}

	if err := h.accountRepo.Update(c.Context(), account); err != nil {
		return responses.InternalError(c, "Could not update account")
	}

	return c.JSON(accountToJSON(account))
}
