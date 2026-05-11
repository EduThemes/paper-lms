package handlers

import (
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"gorm.io/gorm"
)

type SetupHandler struct {
	userService *service.UserService
	accountRepo repository.AccountRepository
	userRepo    repository.UserRepository
	jwtSecret   string
	environment string
	setupMu     sync.Mutex // serializes CompleteSetup to prevent race conditions
}

func NewSetupHandler(userService *service.UserService, accountRepo repository.AccountRepository, userRepo repository.UserRepository, jwtSecret string, environment string) *SetupHandler {
	return &SetupHandler{
		userService: userService,
		accountRepo: accountRepo,
		userRepo:    userRepo,
		jwtSecret:   jwtSecret,
		environment: environment,
	}
}

// hasAdmin paginates through all users to check if any admin exists.
func (h *SetupHandler) hasAdmin(c *fiber.Ctx) (bool, error) {
	page := 1
	perPage := 100
	for {
		result, err := h.userRepo.List(c.Context(), repository.PaginationParams{Page: page, PerPage: perPage})
		if err != nil {
			return false, err
		}
		for _, u := range result.Items {
			if u.Role == "admin" {
				return true, nil
			}
		}
		// If we've seen all users, stop
		if int64(page*perPage) >= result.TotalCount {
			break
		}
		page++
	}
	return false, nil
}

// GetStatus returns whether initial setup has been completed.
func (h *SetupHandler) GetStatus(c *fiber.Ctx) error {
	adminExists, err := h.hasAdmin(c)
	if err != nil {
		return responses.InternalError(c, "Could not check setup status")
	}
	return c.JSON(fiber.Map{
		"setup_complete": adminExists,
	})
}

type setupRequest struct {
	AdminName     string `json:"admin_name"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
	InstanceName  string `json:"instance_name"`
}

// CompleteSetup creates the first admin user and optionally updates the instance name.
// Serialized with a mutex to prevent race conditions where two requests both
// pass the admin-exists guard simultaneously.
func (h *SetupHandler) CompleteSetup(c *fiber.Ctx) error {
	h.setupMu.Lock()
	defer h.setupMu.Unlock()

	// Guard: if an admin already exists, reject
	adminExists, err := h.hasAdmin(c)
	if err != nil {
		return responses.InternalError(c, "Could not check setup status")
	}
	if adminExists {
		return responses.Error(c, fiber.StatusForbidden, "Setup already completed")
	}

	var input setupRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Validate
	input.AdminName = strings.TrimSpace(input.AdminName)
	input.AdminEmail = strings.TrimSpace(input.AdminEmail)
	input.InstanceName = strings.TrimSpace(input.InstanceName)

	if input.AdminName == "" {
		return responses.BadRequest(c, "Admin name is required")
	}
	if input.AdminEmail == "" || !strings.Contains(input.AdminEmail, "@") {
		return responses.BadRequest(c, "A valid email is required")
	}
	if len(input.AdminPassword) < 8 {
		return responses.BadRequest(c, "Password must be at least 8 characters")
	}

	// Create admin user via UserService.Register (handles password hashing)
	user, err := h.userService.Register(c.Context(), input.AdminName, input.AdminEmail, input.AdminPassword)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Set role to admin
	user.Role = "admin"
	if err := h.userRepo.Update(c.Context(), user); err != nil {
		return responses.InternalError(c, "Could not set admin role")
	}

	// Update account name if provided
	if input.InstanceName != "" {
		account, err := h.accountRepo.FindByID(c.Context(), 1)
		if err != nil && err != gorm.ErrRecordNotFound {
			return responses.InternalError(c, "Could not update instance name")
		}
		if account != nil {
			account.Name = input.InstanceName
			if err := h.accountRepo.Update(c.Context(), account); err != nil {
				return responses.InternalError(c, "Could not update instance name")
			}
		}
	}

	return c.JSON(fiber.Map{
		"message": "Setup complete",
		"user": fiber.Map{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}
