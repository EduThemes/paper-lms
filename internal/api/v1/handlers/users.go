package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

type UserHandler struct {
	userService    *service.UserService
	auditService   *service.AuditService
	jwtSecret      string
	environment    string
	tokenBlacklist *service.TokenBlacklist
	// loginPipeline (Phase 9-PRE) — the local-password Login handler
	// produces an SSOOutcome and runs it through the pipeline so MFA
	// policy + audit logging + (future) federation linkage applies
	// uniformly with the federated paths. nil-safe: when not wired
	// the handler falls back to the pre-9-PRE direct-mint flow.
	loginPipeline *auth.LoginPipeline

	// Phase 13.4 (Wave C.2) — COPPA signup gate dependencies. All
	// nil-safe: a nil accountRepo skips the gate entirely (the older
	// development path and existing tests still work). Production
	// wires the full set.
	accountRepo         repository.AccountRepository
	ageVerifyRepo       postgres.AgeVerificationRepository
	parentalConsentRepo postgres.ParentalConsentRepository
	authAudit           *auth.AuthAudit
}

func NewUserHandler(userService *service.UserService, jwtSecret string, environment string, tokenBlacklist *service.TokenBlacklist, auditService *service.AuditService, loginPipeline *auth.LoginPipeline) *UserHandler {
	return &UserHandler{userService: userService, jwtSecret: jwtSecret, environment: environment, tokenBlacklist: tokenBlacklist, auditService: auditService, loginPipeline: loginPipeline}
}

// WithCOPPADeps wires the Phase 13.4 signup gate. Optional builder
// method so the existing NewUserHandler call sites (and tests) keep
// working without a six-arg explosion. Returns the handler for chaining.
func (h *UserHandler) WithCOPPADeps(accountRepo repository.AccountRepository, ageVerifyRepo postgres.AgeVerificationRepository, parentalConsentRepo postgres.ParentalConsentRepository, authAudit *auth.AuthAudit) *UserHandler {
	h.accountRepo = accountRepo
	h.ageVerifyRepo = ageVerifyRepo
	h.parentalConsentRepo = parentalConsentRepo
	h.authAudit = authAudit
	return h
}

// setAuthCookie sets an httpOnly secure cookie with the JWT token.
func (h *UserHandler) setAuthCookie(c *fiber.Ctx, token string) {
	secure := h.environment == "production"
	sameSite := "Lax"
	c.Cookie(&fiber.Cookie{
		Name:     "paper_session",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   86400, // 24 hours, matching JWT expiry
		Expires:  time.Now().Add(24 * time.Hour),
	})
}

// clearAuthCookie removes the session cookie.
func (h *UserHandler) clearAuthCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "paper_session",
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Password              string `json:"password"`
	// ParentalConsentToken (Phase 13.4 / Wave C.2) carries the token a
	// parent received via email after a prior consent request. When
	// the signup tenant is coppa_strict and the user's age verification
	// indicates is_under_13, this token MUST be present and valid
	// (status="granted", consent_type="data_collection"); otherwise
	// the new row is created in pending_parental_consent state.
	ParentalConsentToken  string `json:"parental_consent_token"`
}

func (h *UserHandler) Login(c *fiber.Ctx) error {
	var input loginRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	user, err := h.userService.Authenticate(c.Context(), input.Email, input.Password)
	if err != nil {
		return responses.Error(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	// Phase 9-PRE: route through LoginPipeline so MFA policy / audit
	// logging / future federation linkage applies uniformly. The
	// pipeline returns either a real session token OR a pending-MFA
	// token; this handler translates to HTTP.
	if h.loginPipeline != nil {
		outcome := auth.SSOOutcome{
			ProviderType:    "local",
			ExternalSubject: fmt.Sprintf("%d", user.ID),
			Email:           user.Email,
			EmailVerified:   true, // user authenticated against their own row
			Name:            user.Name,
		}
		meta := auth.RequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
		result, err := h.loginPipeline.Execute(c.Context(), outcome, meta)
		if err != nil {
			return responses.InternalError(c, "Could not complete login")
		}
		// MFA gate: pending token returned, real session withheld.
		if result.PendingToken != "" {
			return c.JSON(fiber.Map{
				"pending_token": result.PendingToken,
				"mfa_required":  true,
			})
		}
		// Token issued. Possibly with a "must enroll" flag.
		h.setAuthCookie(c, result.Token)
		body := fiber.Map{
			"token": result.Token,
			"user": fiber.Map{
				"id":            result.User.ID,
				"name":          result.User.Name,
				"sortable_name": result.User.SortableName,
				"short_name":    result.User.ShortName,
				"login_id":      result.User.LoginID,
				"email":         result.User.Email,
				"avatar_url":    result.User.AvatarURL,
				"locale":        result.User.Locale,
				"role":          result.User.Role,
			},
		}
		if result.MustEnroll {
			body["must_enroll_mfa"] = true
		}
		return c.JSON(body)
	}

	// Fallback: pre-9-PRE direct-mint path. Kept so the handler is
	// safe to wire in stages (pipeline can be added to DI after the
	// handler is constructed in tests).
	token, err := auth.GenerateToken(user, h.jwtSecret)
	if err != nil {
		return responses.InternalError(c, "Could not generate token")
	}
	h.setAuthCookie(c, token)
	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":            user.ID,
			"name":          user.Name,
			"sortable_name": user.SortableName,
			"short_name":    user.ShortName,
			"login_id":      user.LoginID,
			"email":         user.Email,
			"avatar_url":    user.AvatarURL,
			"locale":        user.Locale,
			"role":          user.Role,
		},
	})
}

func (h *UserHandler) Register(c *fiber.Ctx) error {
	var input registerRequest
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	user, err := h.userService.Register(c.Context(), input.Name, input.Email, input.Password)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Phase 13.4 (Wave C.2) — COPPA signup gate. When the tenant is
	// coppa_strict AND the user's age verification indicates is_under_13,
	// require a valid parental_consent_token. Without it, mark the user
	// as pending_parental_consent (no auto-login) and return 201 with a
	// message instructing the user that a parent must verify.
	//
	// The user row is already created at this point (because the older
	// Register flow ran first and we re-use the same email-uniqueness
	// path). We then update the requires_parental_consent flag in-place
	// before issuing — or withholding — the session token.
	meta := auth.RequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
	if h.accountRepo != nil && user.AccountID > 0 {
		if account, accErr := h.accountRepo.FindByID(c.Context(), user.AccountID); accErr == nil && account != nil && account.CoppaStrict {
			isUnder13 := false
			if h.ageVerifyRepo != nil {
				if av, avErr := h.ageVerifyRepo.FindByUserID(c.Context(), user.ID); avErr == nil && av != nil {
					isUnder13 = av.IsUnder13
				}
			}
			if isUnder13 {
				tokenValid := false
				if h.parentalConsentRepo != nil && input.ParentalConsentToken != "" {
					if consent, cErr := h.parentalConsentRepo.FindByToken(c.Context(), input.ParentalConsentToken); cErr == nil && consent != nil {
						if consent.Status == "granted" && consent.ConsentedAt != nil {
							tokenValid = true
						}
					}
				}
				if !tokenValid {
					// Mark the row as pending; the user cannot be auto-logged-in.
					user.RequiresParentalConsent = true
					_ = h.userService.Update(c.Context(), user)
					if h.authAudit != nil {
						h.authAudit.RegistrationCompleted(c.Context(), user.ID, "pending_parental_consent", meta)
					}
					return c.Status(fiber.StatusCreated).JSON(fiber.Map{
						"message": "Account created in pending state. A parent must verify the consent token before this account can be used.",
						"user": fiber.Map{
							"id":                       user.ID,
							"name":                     user.Name,
							"email":                    user.Email,
							"requires_parental_consent": true,
						},
					})
				}
			}
		}
	}

	token, err := auth.GenerateToken(user, h.jwtSecret)
	if err != nil {
		return responses.InternalError(c, "Could not generate token")
	}

	// Set httpOnly cookie for browser-based auth
	h.setAuthCookie(c, token)

	// Phase 13.4 (Wave C.2) — audit symmetry with the login path.
	// Pipeline extension to cover Register is the cleaner long-term
	// fix; firing the audit event manually here closes the gap with
	// minimal blast radius.
	if h.authAudit != nil {
		h.authAudit.RegistrationCompleted(c.Context(), user.ID, "active", meta)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":            user.ID,
			"name":          user.Name,
			"sortable_name": user.SortableName,
			"short_name":    user.ShortName,
			"login_id":      user.LoginID,
			"email":         user.Email,
			"locale":        user.Locale,
			"role":          user.Role,
		},
	})
}

func (h *UserHandler) Logout(c *fiber.Ctx) error {
	// Revoke the current JWT so it cannot be reused until natural expiry
	if h.tokenBlacklist != nil {
		tokenStr := c.Cookies("paper_session")
		if tokenStr == "" {
			if authHeader := c.Get("Authorization"); authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenStr = parts[1]
				}
			}
		}
		if tokenStr != "" {
			h.tokenBlacklist.Revoke(tokenStr, time.Now().Add(24*time.Hour))
		}
	}
	h.clearAuthCookie(c)
	return c.JSON(fiber.Map{"logged_out": true})
}

func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	user, err := h.userService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "user")
	}

	return c.JSON(fiber.Map{
		"id":            user.ID,
		"name":          user.Name,
		"sortable_name": user.SortableName,
		"short_name":    user.ShortName,
		"login_id":      user.LoginID,
		"email":         user.Email,
		"avatar_url":    user.AvatarURL,
		"locale":        user.Locale,
		"time_zone":     user.TimeZone,
		"created_at":    user.CreatedAt,
	})
}

func (h *UserHandler) GetUserProfile(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	user, err := h.userService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "user")
	}

	return c.JSON(fiber.Map{
		"id":            user.ID,
		"name":          user.Name,
		"sortable_name": user.SortableName,
		"short_name":    user.ShortName,
		"login_id":      user.LoginID,
		"primary_email": user.Email,
		"avatar_url":    user.AvatarURL,
		"locale":        user.Locale,
		"time_zone":     user.TimeZone,
	})
}

func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	currentUserID, err := getUserID(c)
	if err != nil {
		return err
	}
	if currentUserID != uint(id) {
		return responses.Error(c, fiber.StatusForbidden, "You can only update your own profile")
	}

	user, err := h.userService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "user")
	}

	var input struct {
		User struct {
			Name     string `json:"name"`
			Locale   string `json:"locale"`
			TimeZone string `json:"time_zone"`
		} `json:"user"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.User.Name != "" {
		user.Name = input.User.Name
	}
	if input.User.Locale != "" {
		user.Locale = input.User.Locale
	}
	if input.User.TimeZone != "" {
		user.TimeZone = input.User.TimeZone
	}

	if err := h.userService.Update(c.Context(), user); err != nil {
		return responses.InternalError(c, "Could not update user")
	}

	return c.JSON(fiber.Map{
		"id":            user.ID,
		"name":          user.Name,
		"sortable_name": user.SortableName,
		"short_name":    user.ShortName,
		"login_id":      user.LoginID,
		"email":         user.Email,
		"avatar_url":    user.AvatarURL,
		"locale":        user.Locale,
		"time_zone":     user.TimeZone,
	})
}

// ChangePassword lets a logged-in user rotate their own password. Requires
// the current password to defend against session-theft → account-takeover.
func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if input.CurrentPassword == "" || input.NewPassword == "" {
		return responses.BadRequest(c, "current_password and new_password are required")
	}
	if err := h.userService.ChangePassword(c.Context(), userID, input.CurrentPassword, input.NewPassword); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"changed": true})
}

// UpdateUserRole sets a user's role. Admin-only at the route level.
func (h *UserHandler) UpdateUserRole(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		Role string `json:"role"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	role := strings.TrimSpace(strings.ToLower(input.Role))
	switch role {
	case "admin", "teacher", "observer", "user":
		// allowed
	default:
		return responses.BadRequest(c, "role must be one of: admin, teacher, observer, user")
	}

	user, err := h.userService.GetByID(c.Context(), uint(id), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "user")
	}
	user.Role = role
	if err := h.userService.Update(c.Context(), user); err != nil {
		return responses.InternalError(c, "Could not update role")
	}
	return c.JSON(fiber.Map{"id": user.ID, "email": user.Email, "role": user.Role})
}

func (h *UserHandler) GetSelf(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}
	// Self lookup: the user-id IS the caller. accountID=0 is safe
	// because we're not crossing tenants — the JWT already attested
	// to this user's identity.
	user, err := h.userService.GetByID(c.Context(), userID, 0)
	if err != nil {
		return responses.NotFound(c, "user")
	}

	result := fiber.Map{
		"id":            user.ID,
		"name":          user.Name,
		"sortable_name": user.SortableName,
		"short_name":    user.ShortName,
		"login_id":      user.LoginID,
		"email":         user.Email,
		"avatar_url":    user.AvatarURL,
		"locale":        user.Locale,
		"time_zone":     user.TimeZone,
		"role":          user.Role,
	}

	// If masquerading, include masquerade info so the frontend can show the banner
	if masqueradeByID, ok := c.Locals("masquerade_by").(uint); ok && masqueradeByID > 0 {
		result["masquerading_as"] = user.Name
		result["real_user_id"] = masqueradeByID
		// Look up the real admin user to return their name for display.
		// admin_account_id Locals carries the impersonator's home
		// tenant (see auth middleware 13.1.B + GenerateMasqueradeToken).
		adminAcctID, _ := c.Locals("admin_account_id").(uint)
		adminUser, adminErr := h.userService.GetByID(c.Context(), masqueradeByID, adminAcctID)
		if adminErr == nil {
			result["real_user_name"] = adminUser.Name
		}
	}

	return c.JSON(result)
}

// StartMasquerade allows an admin to start masquerading as a target user.
// POST /api/v1/users/:id/masquerade
func (h *UserHandler) StartMasquerade(c *fiber.Ctx) error {
	adminUserID, err := getUserID(c)
	if err != nil {
		return err
	}

	targetUserID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	// Prevent masquerading as yourself
	if uint(targetUserID) == adminUserID {
		return responses.BadRequest(c, "Cannot masquerade as yourself")
	}

	// Prevent nested masquerade — if already masquerading, block
	if masqueradeByID, ok := c.Locals("masquerade_by").(uint); ok && masqueradeByID > 0 {
		return responses.BadRequest(c, "Cannot start a masquerade while already masquerading. End the current masquerade first.")
	}

	// Look up the target user, scoped to the masquerading admin's
	// tenant. An admin cannot masquerade across tenant boundaries —
	// cross-tenant attempts surface as 404 (existence-leak contract).
	targetUser, err := h.userService.GetByID(c.Context(), uint(targetUserID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "user")
	}

	// 13.1.B — pass the admin's home tenant so the masquerade token's
	// admin_account_id claim attributes the impersonation correctly.
	adminAccountID, _ := c.Locals("account_id").(uint)

	// Generate a masquerade token (target user's identity + admin's ID in masquerade_by claim)
	token, err := auth.GenerateMasqueradeToken(targetUser, adminUserID, adminAccountID, h.jwtSecret)
	if err != nil {
		return responses.InternalError(c, "Could not generate masquerade token")
	}

	// Set the session cookie with the masquerade token
	h.setAuthCookie(c, token)

	// Create an audit log entry
	if h.auditService != nil {
		_ = h.auditService.LogEvent(
			c.Context(),
			"masquerade_start",
			adminUserID,
			nil, // courseID
			nil, // accountID
			"User",
			targetUser.ID,
			fmt.Sprintf("Admin user %d started masquerading as user %d (%s)", adminUserID, targetUser.ID, targetUser.Name),
			fmt.Sprintf(`{"admin_user_id":%d,"target_user_id":%d,"target_user_name":"%s"}`, adminUserID, targetUser.ID, targetUser.Name),
			c.IP(),
			c.Get("User-Agent"),
		)
	}

	return c.JSON(fiber.Map{
		"masquerading": true,
		"user": fiber.Map{
			"id":            targetUser.ID,
			"name":          targetUser.Name,
			"sortable_name": targetUser.SortableName,
			"short_name":    targetUser.ShortName,
			"login_id":      targetUser.LoginID,
			"email":         targetUser.Email,
			"avatar_url":    targetUser.AvatarURL,
			"locale":        targetUser.Locale,
			"role":          targetUser.Role,
		},
	})
}

// EndMasquerade stops masquerading and restores the admin's session.
// DELETE /api/v1/masquerade
func (h *UserHandler) EndMasquerade(c *fiber.Ctx) error {
	// Check that we are actually masquerading
	masqueradeByID, ok := c.Locals("masquerade_by").(uint)
	if !ok || masqueradeByID == 0 {
		return responses.BadRequest(c, "Not currently masquerading")
	}

	// Look up the original admin user. admin_account_id Locals
	// carries the impersonator's home tenant — that's the right
	// scope for this lookup (the masquerade session's account_id
	// Locals belongs to the target, not the admin).
	adminAcctID, _ := c.Locals("admin_account_id").(uint)
	adminUser, err := h.userService.GetByID(c.Context(), masqueradeByID, adminAcctID)
	if err != nil {
		return responses.InternalError(c, "Could not find the original admin user")
	}

	// Generate a normal token for the admin user (no masquerade_by claim)
	token, err := auth.GenerateToken(adminUser, h.jwtSecret)
	if err != nil {
		return responses.InternalError(c, "Could not generate token")
	}

	// Revoke the masquerade token
	if h.tokenBlacklist != nil {
		tokenStr := c.Cookies("paper_session")
		if tokenStr != "" {
			h.tokenBlacklist.Revoke(tokenStr, time.Now().Add(24*time.Hour))
		}
	}

	// Set the session cookie back to admin
	h.setAuthCookie(c, token)

	// Get the masqueraded user's ID for audit logging
	masqueradedUserID, _ := getUserID(c)

	// Create an audit log entry
	if h.auditService != nil {
		_ = h.auditService.LogEvent(
			c.Context(),
			"masquerade_end",
			masqueradeByID,
			nil, // courseID
			nil, // accountID
			"User",
			masqueradedUserID,
			fmt.Sprintf("Admin user %d stopped masquerading as user %d", masqueradeByID, masqueradedUserID),
			fmt.Sprintf(`{"admin_user_id":%d,"target_user_id":%d}`, masqueradeByID, masqueradedUserID),
			c.IP(),
			c.Get("User-Agent"),
		)
	}

	return c.JSON(fiber.Map{
		"masquerading": false,
		"user": fiber.Map{
			"id":            adminUser.ID,
			"name":          adminUser.Name,
			"sortable_name": adminUser.SortableName,
			"short_name":    adminUser.ShortName,
			"login_id":      adminUser.LoginID,
			"email":         adminUser.Email,
			"avatar_url":    adminUser.AvatarURL,
			"locale":        adminUser.Locale,
			"role":          adminUser.Role,
		},
	})
}

func (h *UserHandler) RequestPasswordReset(c *fiber.Ctx) error {
	var input struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Always return success to avoid email enumeration
	_, _ = h.userService.RequestPasswordReset(c.Context(), input.Email)
	return c.JSON(fiber.Map{"requested": true})
}

func (h *UserHandler) ResetPassword(c *fiber.Ctx) error {
	var input struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if err := h.userService.ResetPassword(c.Context(), input.Token, input.NewPassword); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"password_reset": true})
}

func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	params := middleware.GetPagination(c)
	searchTerm := c.Query("search_term")

	var result *repository.PaginatedResult[models.User]
	var err error
	if searchTerm != "" {
		// 13.1.D: scope user search to the caller's tenant so an admin
		// in one account cannot enumerate users in another via name or
		// email substring.
		result, err = h.userService.Search(c.Context(), searchTerm, callerAccountID(c), params)
	} else {
		// Wave 2 widening: List scopes to the caller's tenant for
		// the same reason Search does.
		result, err = h.userService.List(c.Context(), params, callerAccountID(c))
	}
	if err != nil {
		return responses.InternalError(c, "Could not fetch users")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	users := make([]fiber.Map, len(result.Items))
	for i, u := range result.Items {
		users[i] = fiber.Map{
			"id":            u.ID,
			"name":          u.Name,
			"sortable_name": u.SortableName,
			"short_name":    u.ShortName,
			"login_id":      u.LoginID,
			"email":         u.Email,
			"avatar_url":    u.AvatarURL,
			"locale":        u.Locale,
			"role":          u.Role,
			"created_at":    u.CreatedAt,
		}
	}

	return c.JSON(users)
}
