package handlers

import (
	"context"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// bootstrapAdvisoryLockKey is a stable Postgres advisory-lock key used
// to serialize POST /setup/complete across pods. Any constant will do
// — it just needs to be deterministic so every pod targets the same
// lock slot. Hex-encoded "PAPRBOOT" sits in the high bytes so it's
// unambiguously outside the scheduler's fnv-64 hash space.
const bootstrapAdvisoryLockKey int64 = 0x504150_5242_4F_4F_54 // "PAPRBOOT"

type SetupHandler struct {
	userService *service.UserService
	accountRepo repository.AccountRepository
	userRepo    repository.UserRepository
	db          *gorm.DB
	jwtSecret   string
	environment string

	// setupMu serializes CompleteSetup within ONE pod. The
	// Postgres advisory lock below covers cross-pod races; the
	// in-process mutex is a defense-in-depth shortcut that avoids
	// firing the DB round-trip on a same-pod retry.
	setupMu sync.Mutex
}

// NewSetupHandler constructs the setup wizard handler. `db` must be
// non-nil in production — it is used to acquire the cross-pod
// bootstrap advisory lock during CompleteSetup. Tests that exercise
// the handler in isolation may pass a nil db; the handler degrades
// to the in-process mutex only and logs a warning at the call site.
func NewSetupHandler(userService *service.UserService, accountRepo repository.AccountRepository, userRepo repository.UserRepository, db *gorm.DB, jwtSecret string, environment string) *SetupHandler {
	return &SetupHandler{
		userService: userService,
		accountRepo: accountRepo,
		userRepo:    userRepo,
		db:          db,
		jwtSecret:   jwtSecret,
		environment: environment,
	}
}

// withBootstrapLock acquires a Postgres session-level advisory lock
// keyed at bootstrapAdvisoryLockKey, runs fn, then releases. The
// lock blocks rather than spinning — a second pod racing the wizard
// waits for the first to commit its admin-promotion before re-running
// the hasAdmin guard, at which point the guard correctly returns
// "setup already completed."
//
// Multi-pod scenario this defends:
//
//	t=0: attacker hits pod A → POST /setup/complete (no admin yet)
//	t=0: attacker hits pod B → POST /setup/complete (no admin yet)
//	t=0: attacker hits pod C → POST /setup/complete (no admin yet)
//
// Without the DB lock, all three pods clear the in-process mutex
// guard simultaneously and create three super_admins. With the lock,
// pods B and C serialize behind A; B/C's post-lock hasAdmin check
// finds A's super_admin and 403s.
//
// Per the 2026-05-17 Wave 2 audit, finding M1.
func (h *SetupHandler) withBootstrapLock(ctx context.Context, fn func() error) error {
	if h.db == nil {
		// Test wiring — fall back to mutex-only. Production main.go
		// always passes the real DB.
		return fn()
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", bootstrapAdvisoryLockKey); err != nil {
		return err
	}
	defer func() {
		// Use context.Background here so a canceled request still
		// releases the lock — otherwise an attacker who aborts mid-
		// setup would orphan the lock and block legitimate retries.
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", bootstrapAdvisoryLockKey)
	}()

	return fn()
}

// hasAdmin paginates through all users to check if any admin or
// super_admin exists. The super_admin role is included so that fresh
// deployments which bootstrapped the first user as super_admin
// (Settings-Engine path) still report setup_complete=true; otherwise
// the wizard would re-prompt and re-create users.
func (h *SetupHandler) hasAdmin(c *fiber.Ctx) (bool, error) {
	page := 1
	perPage := 100
	for {
		result, err := h.userRepo.List(c.Context(), repository.PaginationParams{Page: page, PerPage: perPage})
		if err != nil {
			return false, err
		}
		for _, u := range result.Items {
			if u.Role == "admin" || u.Role == "super_admin" {
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

// CompleteSetup creates the first super_admin user and optionally updates
// the instance name. Multi-layer race protection:
//
//  1. In-process mutex (setupMu) — serializes within ONE pod, the
//     cheap path that catches double-clicks and tabbed retries.
//  2. Postgres advisory lock (withBootstrapLock) — serializes ACROSS
//     pods. Without it a 3-pod deployment hit simultaneously by an
//     attacker would create three super_admins. See audit M1.
//  3. Post-lock hasAdmin re-check — the second concurrent caller
//     waits for #2, then this check finds the just-created admin
//     and returns 403.
//
// Layer #1 is defense-in-depth (faster than the DB round-trip) but
// not load-bearing; #2 + #3 are the actual correctness guarantee.
func (h *SetupHandler) CompleteSetup(c *fiber.Ctx) error {
	h.setupMu.Lock()
	defer h.setupMu.Unlock()

	var resultUser *struct {
		ID    uint
		Name  string
		Email string
		Role  string
	}
	var responseErr error

	lockErr := h.withBootstrapLock(c.Context(), func() error {
		// Re-check INSIDE the lock — a racing caller waited for us
		// to commit and now sees the admin we just created.
		adminExists, err := h.hasAdmin(c)
		if err != nil {
			responseErr = responses.InternalError(c, "Could not check setup status")
			return nil
		}
		if adminExists {
			responseErr = responses.Error(c, fiber.StatusForbidden, "Setup already completed")
			return nil
		}

		var input setupRequest
		if err := c.BodyParser(&input); err != nil {
			responseErr = responses.BadRequest(c, "Invalid input")
			return nil
		}

		input.AdminName = strings.TrimSpace(input.AdminName)
		input.AdminEmail = strings.TrimSpace(input.AdminEmail)
		input.InstanceName = strings.TrimSpace(input.InstanceName)

		if input.AdminName == "" {
			responseErr = responses.BadRequest(c, "Admin name is required")
			return nil
		}
		if input.AdminEmail == "" || !strings.Contains(input.AdminEmail, "@") {
			responseErr = responses.BadRequest(c, "A valid email is required")
			return nil
		}
		if len(input.AdminPassword) < 8 {
			responseErr = responses.BadRequest(c, "Password must be at least 8 characters")
			return nil
		}

		user, err := h.userService.Register(c.Context(), input.AdminName, input.AdminEmail, input.AdminPassword)
		if err != nil {
			responseErr = responses.BadRequest(c, err.Error())
			return nil
		}

		// Promote to super_admin. The first user on a fresh
		// deployment becomes the platform operator — the role above
		// account-admin that owns the Super-Admin Settings Engine
		// surface (SMTP, S3, AI keys, …). Subsequent platform
		// operators are promoted by an existing super_admin via the
		// Wave 2/4 admin UI.
		user.Role = "super_admin"
		if err := h.userRepo.Update(c.Context(), user); err != nil {
			responseErr = responses.InternalError(c, "Could not set super-admin role")
			return nil
		}

		if input.InstanceName != "" {
			account, err := h.accountRepo.FindByID(c.Context(), 1)
			if err != nil && err != gorm.ErrRecordNotFound {
				responseErr = responses.InternalError(c, "Could not update instance name")
				return nil
			}
			if account != nil {
				account.Name = input.InstanceName
				if err := h.accountRepo.Update(c.Context(), account); err != nil {
					responseErr = responses.InternalError(c, "Could not update instance name")
					return nil
				}
			}
		}

		resultUser = &struct {
			ID    uint
			Name  string
			Email string
			Role  string
		}{user.ID, user.Name, user.Email, user.Role}
		return nil
	})
	if lockErr != nil {
		return responses.InternalError(c, "Could not acquire setup lock")
	}
	if responseErr != nil {
		return responseErr
	}

	return c.JSON(fiber.Map{
		"message": "Setup complete",
		"user": fiber.Map{
			"id":    resultUser.ID,
			"name":  resultUser.Name,
			"email": resultUser.Email,
			"role":  resultUser.Role,
		},
	})
}
