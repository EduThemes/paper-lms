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
		"id":                  a.ID,
		"name":                a.Name,
		"parent_account_id":   a.ParentAccountID,
		"root_account_id":     a.RootAccountID,
		"workflow_state":      a.WorkflowState,
		"max_upload_size_mb":  a.MaxUploadSizeMB,
	}
}

func (h *AccountHandler) ListAccounts(c *fiber.Ctx) error {
	params := middleware.GetPagination(c)

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

func (h *AccountHandler) GetAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	account, err := h.accountRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "account")
	}

	return c.JSON(accountToJSON(account))
}

// UpdateAccount lets an admin edit account-level settings.
// Currently exposes name and max_upload_size_mb.
func (h *AccountHandler) UpdateAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	account, err := h.accountRepo.FindByID(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "account")
	}

	var input struct {
		Name     *string `json:"name"`
		Settings *struct {
			MaxUploadSizeMB *uint `json:"max_upload_size_mb"`
		} `json:"settings"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Name != nil && *input.Name != "" {
		account.Name = *input.Name
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
