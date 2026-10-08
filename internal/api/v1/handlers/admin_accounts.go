package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// AdminAccountsHandler serves the account-administration lane: admin user
// management (G114), additional logins (G115), account-level observer links
// (G121), user logs (G119), account reports (G117), theme (G118), admin
// tools (G119), storage quotas (G120) and the custom-role helpers (G102).
// Every account-scoped route is mounted behind RequireAdmin, which ties
// :account_id to the caller's tenant; the handlers re-check it with
// sisAccountScope (404 across tenants, super_admin excepted).
type AdminAccountsHandler struct {
	users     *service.AdminUserService
	logins    *service.UserLoginService
	observers AdminObserverLinker
	reports   *service.AccountReportService
	brand     *service.BrandConfigService
	tools     *service.AdminToolsService
	quotas    *service.StorageQuotaService
	roles     *service.CustomRoleService
	audit     *service.AuditService
}

// AdminObserverLinker pairs observers with students (*service.ObserverService).
type AdminObserverLinker interface {
	LinkObserverToStudent(ctx context.Context, observerUserID, studentUserID uint) error
	UnlinkObserver(ctx context.Context, observerUserID, studentUserID uint) error
}

// AdminAccountsDeps bundles the handler's services.
type AdminAccountsDeps struct {
	Users     *service.AdminUserService
	Logins    *service.UserLoginService
	Observers AdminObserverLinker
	Reports   *service.AccountReportService
	Brand     *service.BrandConfigService
	Tools     *service.AdminToolsService
	Quotas    *service.StorageQuotaService
	Roles     *service.CustomRoleService
	Audit     *service.AuditService
}

// NewAdminAccountsHandler builds the handler.
func NewAdminAccountsHandler(d AdminAccountsDeps) *AdminAccountsHandler {
	return &AdminAccountsHandler{
		users: d.Users, logins: d.Logins, observers: d.Observers, reports: d.Reports,
		brand: d.Brand, tools: d.Tools, quotas: d.Quotas, roles: d.Roles, audit: d.Audit,
	}
}

// targetUser resolves :user_id inside the URL account. wrote=true means a
// response (400/404) was written and the caller must return nil.
func (h *AdminAccountsHandler) targetUser(c *fiber.Ctx) (*models.User, uint, bool) {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil, 0, true
	}
	id, err := c.ParamsInt("user_id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid user ID")
		return nil, 0, true
	}
	u, err := h.users.GetUser(c.Context(), uint(id), accountID)
	if err != nil {
		_ = responses.NotFound(c, "user")
		return nil, 0, true
	}
	return u, accountID, false
}

// adminUserJSON renders a user for the admin pages.
func adminUserJSON(u *models.User) fiber.Map {
	return fiber.Map{
		"id":                      u.ID,
		"account_id":              u.AccountID,
		"name":                    u.Name,
		"short_name":              u.ShortName,
		"sortable_name":           u.SortableName,
		"login_id":                u.LoginID,
		"sis_user_id":             u.SISUserID,
		"email":                   u.Email,
		"role":                    u.Role,
		"locale":                  u.Locale,
		"time_zone":               u.TimeZone,
		"suspended":               u.Suspended,
		"suspended_by_sis":        u.SuspendedBySIS,
		"requires_password_reset": u.RequiresPasswordReset,
		"mfa_enrolled":            u.TOTPVerifiedAt != nil,
		"created_at":              u.CreatedAt,
	}
}

type adminUserBody struct {
	Name                 *string `json:"name"`
	ShortName            *string `json:"short_name"`
	SortableName         *string `json:"sortable_name"`
	Email                *string `json:"email"`
	LoginID              *string `json:"login_id"`
	SISUserID            *string `json:"sis_user_id"`
	TimeZone             *string `json:"time_zone"`
	Locale               *string `json:"locale"`
	Password             *string `json:"password"`
	RequirePasswordReset *bool   `json:"require_password_reset"`
}

func (b adminUserBody) input() service.AdminUserInput {
	return service.AdminUserInput{
		Name: b.Name, ShortName: b.ShortName, SortableName: b.SortableName, Email: b.Email,
		LoginID: b.LoginID, SISUserID: b.SISUserID, TimeZone: b.TimeZone, Locale: b.Locale,
		Password: b.Password, RequirePasswordReset: b.RequirePasswordReset,
	}
}

func adminUserError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrAdminUserLoginTaken), errors.Is(err, service.ErrAdminUserSISTaken):
		return responses.Error(c, fiber.StatusConflict, err.Error())
	case errors.Is(err, service.ErrAdminUserNotFound):
		return responses.NotFound(c, "user")
	}
	return responses.BadRequest(c, err.Error())
}

// CreateUser handles POST /accounts/:account_id/users (G114).
func (h *AdminAccountsHandler) CreateUser(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}
	var body struct {
		User adminUserBody `json:"user"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	u, err := h.users.CreateUser(c.Context(), accountID, body.User.input())
	if err != nil {
		return adminUserError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(adminUserJSON(u))
}

// GetUserDetail handles GET /accounts/:account_id/users/:user_id: the
// profile plus enrollments, logins and observer links.
func (h *AdminAccountsHandler) GetUserDetail(c *fiber.Ctx) error {
	u, accountID, wrote := h.targetUser(c)
	if wrote {
		return nil
	}
	ctx := c.Context()
	enrollments, err := h.users.ListEnrollments(ctx, u.ID, accountID)
	if err != nil {
		return responses.InternalError(c, "Could not load enrollments")
	}
	logins, err := h.logins.List(ctx, u.ID, accountID)
	if err != nil {
		return responses.InternalError(c, "Could not load logins")
	}
	observees, _ := h.users.ListObservees(ctx, u.ID, accountID)
	observers, _ := h.users.ListObservers(ctx, u.ID, accountID)
	if enrollments == nil {
		enrollments = []service.AdminUserEnrollment{}
	}
	if observees == nil {
		observees = []service.ObserverLinkRow{}
	}
	if observers == nil {
		observers = []service.ObserverLinkRow{}
	}

	callerID, _ := c.Locals("user_id").(uint)
	callerRole := middleware.EffectiveRole(c, nil)
	_, masquerading := c.Locals("masquerade_by").(uint)
	canManage := callerMayManageUser(c, u)
	out := adminUserJSON(u)
	out["enrollments"] = enrollments
	out["logins"] = userLoginsJSON(logins)
	out["observees"] = observees
	out["observers"] = observers
	out["permissions"] = fiber.Map{
		"can_manage": canManage,
		// Canvas can_masquerade?: never yourself, never someone with more
		// rights than you, never from inside a masquerade.
		"can_masquerade": u.ID != callerID && !masquerading &&
			middleware.RoleRank(u.Role) <= middleware.RoleRank(callerRole),
	}
	return c.JSON(out)
}

// UpdateUser handles PUT /accounts/:account_id/users/:user_id (G114): an
// admin edits another user's name, email, primary login, SIS id, time
// zone or password.
func (h *AdminAccountsHandler) UpdateUser(c *fiber.Ctx) error {
	u, _, wrote := h.targetUser(c)
	if wrote {
		return nil
	}
	if !callerMayManageUser(c, u) {
		return responses.Forbidden(c, "Only a super_admin can change a super_admin")
	}
	var body struct {
		User adminUserBody `json:"user"`
	}
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if err := h.users.UpdateUser(c.Context(), u, body.User.input()); err != nil {
		return adminUserError(c, err)
	}
	return c.JSON(adminUserJSON(u))
}

// GetUserHistory handles GET /accounts/:account_id/users/:user_id/history
// (G119): sign-in history, audited activity and grade changes.
func (h *AdminAccountsHandler) GetUserHistory(c *fiber.Ctx) error {
	u, accountID, wrote := h.targetUser(c)
	if wrote {
		return nil
	}
	ctx := c.Context()
	limit := c.QueryInt("limit", 50)
	logins, err := h.users.ListAuthEvents(ctx, u.ID, limit)
	if err != nil {
		return responses.InternalError(c, "Could not load sign-in history")
	}
	activity, err := h.users.ListUserActivity(ctx, u.ID, accountID, limit)
	if err != nil {
		return responses.InternalError(c, "Could not load activity")
	}
	grades, err := h.users.ListGradeChanges(ctx, u.ID, accountID, limit)
	if err != nil {
		return responses.InternalError(c, "Could not load grade changes")
	}
	authOut := make([]fiber.Map, len(logins))
	for i := range logins {
		authOut[i] = auditLogToJSON(&logins[i])
	}
	actOut := make([]fiber.Map, len(activity))
	for i := range activity {
		actOut[i] = auditLogToJSON(&activity[i])
	}
	return c.JSON(fiber.Map{"logins": authOut, "activity": actOut, "grade_changes": grades})
}

// ---------------------------------------------------------------------------
// Logins (G115)
// ---------------------------------------------------------------------------

func userLoginJSON(l *models.UserLogin) fiber.Map {
	return fiber.Map{
		"id":                         l.ID,
		"user_id":                    l.UserID,
		"account_id":                 l.AccountID,
		"unique_id":                  l.UniqueID,
		"sis_user_id":                l.SISUserID,
		"authentication_provider_id": l.AuthenticationProviderID,
		"has_password":               l.HasPassword(),
		"workflow_state":             l.WorkflowState,
		"last_login_at":              l.LastLoginAt,
		"created_at":                 l.CreatedAt,
	}
}

func userLoginsJSON(ls []models.UserLogin) []fiber.Map {
	out := make([]fiber.Map, len(ls))
	for i := range ls {
		out[i] = userLoginJSON(&ls[i])
	}
	return out
}

type loginBody struct {
	Login struct {
		UniqueID                 *string `json:"unique_id"`
		Password                 *string `json:"password"`
		SISUserID                *string `json:"sis_user_id"`
		AuthenticationProviderID *uint   `json:"authentication_provider_id"`
		WorkflowState            *string `json:"workflow_state"`
	} `json:"login"`
}

func (b loginBody) input() service.UserLoginInput {
	return service.UserLoginInput{
		UniqueID: b.Login.UniqueID, Password: b.Login.Password, SISUserID: b.Login.SISUserID,
		AuthenticationProviderID: b.Login.AuthenticationProviderID, WorkflowState: b.Login.WorkflowState,
	}
}

func loginError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrUserLoginNotFound):
		return responses.NotFound(c, "login")
	case errors.Is(err, service.ErrUserLoginTaken), errors.Is(err, service.ErrUserLoginSISTaken):
		return responses.Error(c, fiber.StatusConflict, err.Error())
	}
	return responses.BadRequest(c, err.Error())
}

// ListLogins handles GET /accounts/:account_id/users/:user_id/logins.
func (h *AdminAccountsHandler) ListLogins(c *fiber.Ctx) error {
	u, accountID, wrote := h.targetUser(c)
	if wrote {
		return nil
	}
	ls, err := h.logins.List(c.Context(), u.ID, accountID)
	if err != nil {
		return loginError(c, err)
	}
	return c.JSON(userLoginsJSON(ls))
}

// CreateLogin handles POST /accounts/:account_id/users/:user_id/logins.
func (h *AdminAccountsHandler) CreateLogin(c *fiber.Ctx) error {
	u, accountID, wrote := h.targetUser(c)
	if wrote {
		return nil
	}
	if !callerMayManageUser(c, u) {
		return responses.Forbidden(c, "Only a super_admin can change a super_admin")
	}
	var body loginBody
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	l, err := h.logins.Create(c.Context(), u.ID, accountID, body.input())
	if err != nil {
		return loginError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(userLoginJSON(l))
}

// UpdateLogin handles PUT /accounts/:account_id/users/:user_id/logins/:login_id.
func (h *AdminAccountsHandler) UpdateLogin(c *fiber.Ctx) error {
	u, accountID, wrote := h.targetUser(c)
	if wrote {
		return nil
	}
	if !callerMayManageUser(c, u) {
		return responses.Forbidden(c, "Only a super_admin can change a super_admin")
	}
	loginID, err := c.ParamsInt("login_id")
	if err != nil || loginID <= 0 {
		return responses.BadRequest(c, "Invalid login ID")
	}
	var body loginBody
	if err := c.BodyParser(&body); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	l, err := h.logins.Update(c.Context(), u.ID, uint(loginID), accountID, body.input())
	if err != nil {
		return loginError(c, err)
	}
	return c.JSON(userLoginJSON(l))
}

// DeleteLogin handles DELETE /accounts/:account_id/users/:user_id/logins/:login_id.
func (h *AdminAccountsHandler) DeleteLogin(c *fiber.Ctx) error {
	u, accountID, wrote := h.targetUser(c)
	if wrote {
		return nil
	}
	if !callerMayManageUser(c, u) {
		return responses.Forbidden(c, "Only a super_admin can change a super_admin")
	}
	loginID, err := c.ParamsInt("login_id")
	if err != nil || loginID <= 0 {
		return responses.BadRequest(c, "Invalid login ID")
	}
	if err := h.logins.Delete(c.Context(), u.ID, uint(loginID), accountID); err != nil {
		return loginError(c, err)
	}
	return c.JSON(fiber.Map{"id": loginID, "deleted": true})
}

// ---------------------------------------------------------------------------
// Observer links (G121)
// ---------------------------------------------------------------------------

// observerPair resolves :user_id (observer) and :observee_id (student),
// both inside the URL account.
func (h *AdminAccountsHandler) observerPair(c *fiber.Ctx) (*models.User, *models.User, bool) {
	observer, accountID, wrote := h.targetUser(c)
	if wrote {
		return nil, nil, true
	}
	sid, err := c.ParamsInt("observee_id")
	if err != nil || sid <= 0 {
		_ = responses.BadRequest(c, "Invalid observee ID")
		return nil, nil, true
	}
	student, err := h.users.GetUser(c.Context(), uint(sid), accountID)
	if err != nil {
		_ = responses.NotFound(c, "user")
		return nil, nil, true
	}
	if student.ID == observer.ID {
		_ = responses.BadRequest(c, "A user can't observe themselves")
		return nil, nil, true
	}
	return observer, student, false
}

// LinkObservee handles PUT /accounts/:account_id/users/:user_id/observees/:observee_id:
// an admin pairs an observer with a student directly (Canvas: no pairing
// code needed for an account admin). The link is durable: the observer
// follows the student into every current and future course.
func (h *AdminAccountsHandler) LinkObservee(c *fiber.Ctx) error {
	observer, student, wrote := h.observerPair(c)
	if wrote {
		return nil
	}
	if err := h.observers.LinkObserverToStudent(c.Context(), observer.ID, student.ID); err != nil {
		if strings.Contains(err.Error(), "already linked") {
			return c.JSON(fiber.Map{"observer_id": observer.ID, "observee_id": student.ID, "linked": true})
		}
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"observer_id": observer.ID, "observee_id": student.ID, "linked": true})
}

// UnlinkObservee handles DELETE /accounts/:account_id/users/:user_id/observees/:observee_id.
func (h *AdminAccountsHandler) UnlinkObservee(c *fiber.Ctx) error {
	observer, student, wrote := h.observerPair(c)
	if wrote {
		return nil
	}
	if err := h.observers.UnlinkObserver(c.Context(), observer.ID, student.ID); err != nil {
		return responses.NotFound(c, "observer link")
	}
	return c.JSON(fiber.Map{"observer_id": observer.ID, "observee_id": student.ID, "linked": false})
}
