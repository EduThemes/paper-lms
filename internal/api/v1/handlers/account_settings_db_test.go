package handlers_test

// DB-backed tests for the account-admin settings API
// (GET/PUT /api/v1/accounts/:account_id/settings) and the
// "off anywhere above wins" resolution of pages.custom_css_enabled that
// course show reports. All data is made up.

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/service/settings"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const cssSettingKey = settings.PagesCustomCSSEnabled

type accountSettingsFixture struct {
	g   *gorm.DB
	svc *settings.Service
	h   *handlers.AccountSettingsHandler
	pm  *middleware.PermissionMiddleware
}

// Accounts: 4 "Example District" is the parent of 2 (coreScratchDB's
// tenant, admin user 3); 1 is an unrelated tenant. Users: 30 is a plain
// user (a teacher) in account 2, 40 a super-admin.
func newAccountSettingsFixture(t *testing.T) *accountSettingsFixture {
	t.Helper()
	g := coreScratchDB(t)
	require.NoError(t, g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (4, 'Example District', 'active', 'off', 'en', 'higher_ed', 500)`).Error)
	require.NoError(t, g.Exec(`UPDATE accounts SET parent_account_id = 4, name = 'Example School' WHERE id = 2`).Error)
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: 30, AccountID: 2, Name: "Teacher Example", LoginID: "teacher@example.test", Email: "teacher@example.test", PasswordHash: "x", Role: "user"},
		{ID: 40, AccountID: 1, Name: "Operator Example", LoginID: "ops@example.test", Email: "ops@example.test", PasswordHash: "x", Role: "super_admin"},
	} {
		require.NoError(t, users.Create(context.Background(), u))
	}

	audit := service.NewAuditService(postgres.NewAuditLogRepository(g), postgres.NewGradeChangeLogRepository(g), postgres.NewPIIAccessLogRepository(g))
	accounts := postgres.NewAccountRepository(g)
	svc := settings.NewService(postgres.NewSettingRepository(g), accounts, audit)
	svc.SetEnvReader(func(string) string { return "" })
	return &accountSettingsFixture{
		g:   g,
		svc: svc,
		h:   handlers.NewAccountSettingsHandler(svc, accounts),
		pm:  middleware.NewPermissionMiddleware(postgres.NewEnrollmentRepository(g), users, postgres.NewCourseRepository(g)),
	}
}

// app mounts the routes behind the real RequireAdmin, as the router does.
func (f *accountSettingsFixture) app(userID, accountID uint) *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		return c.Next()
	})
	app.Get("/accounts/:account_id/settings", f.pm.RequireAdmin(), f.h.List)
	app.Put("/accounts/:account_id/settings/:key", f.pm.RequireAdmin(), f.h.Update)
	return app
}

func (f *accountSettingsFixture) set(t *testing.T, scope settings.ScopeType, id uint, v string) {
	t.Helper()
	require.NoError(t, f.svc.Set(context.Background(), scope, id, cssSettingKey, v, 40))
}

func putSetting(app *fiber.App, account uint, key string, value any) *http.Response {
	return testutil.MakeRequest(app, http.MethodPut, fmt.Sprintf("/accounts/%d/settings/%s", account, key),
		testutil.JSONBody(map[string]any{"value": value}))
}

func listSettings(t *testing.T, app *fiber.App, account uint) (int, map[string]any) {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/accounts/%d/settings", account), nil)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, items, 1, "only the allow-listed key")
	return resp.StatusCode, items[0]
}

func TestAccountSettings_TenantAdminReadsAndWritesOwnAccount(t *testing.T) {
	f := newAccountSettingsFixture(t)
	admin := f.app(3, 2)

	status, item := listSettings(t, admin, 2)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, cssSettingKey, item["key"])
	assert.NotEmpty(t, item["label"])
	assert.Nil(t, item["value"], "inherits")
	assert.Equal(t, true, item["effective"], "catalog default")
	assert.Equal(t, false, item["locked"])
	assert.Nil(t, item["locked_by"])

	resp := putSetting(admin, 2, cssSettingKey, false)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := jsonMap(t, resp)
	assert.Equal(t, false, got["value"])
	assert.Equal(t, false, got["effective"])
	assert.Equal(t, false, got["locked"], "the account's own off is not a lock")

	resp = putSetting(admin, 2, cssSettingKey, true)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, jsonMap(t, resp)["effective"])

	// null clears the account's row: back to inheriting.
	resp = putSetting(admin, 2, cssSettingKey, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got = jsonMap(t, resp)
	assert.Nil(t, got["value"])
	assert.Equal(t, true, got["effective"])
	var rows int64
	require.NoError(t, f.g.Model(&models.Setting{}).Where("scope_type = 'account' AND scope_id = 2 AND key = ?", cssSettingKey).Count(&rows).Error)
	assert.Zero(t, rows)

	// Each write left a setting_change audit row by the admin.
	var audits int64
	require.NoError(t, f.g.Table("audit_logs").Where("event_type = 'setting_change' AND user_id = 3").Count(&audits).Error)
	assert.EqualValues(t, 3, audits)
}

func TestAccountSettings_AccessRules(t *testing.T) {
	f := newAccountSettingsFixture(t)
	admin := f.app(3, 2)

	// Another tenant's account: 404, read and write.
	status, _ := listSettings(t, admin, 1)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, http.StatusNotFound, putSetting(admin, 1, cssSettingKey, false).StatusCode)
	// Its parent account is another account too (no sub-account admin yet).
	assert.Equal(t, http.StatusNotFound, putSetting(admin, 4, cssSettingKey, false).StatusCode)

	// A teacher (not an admin): 403.
	teacher := f.app(30, 2)
	status, _ = listSettings(t, teacher, 2)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, http.StatusForbidden, putSetting(teacher, 2, cssSettingKey, false).StatusCode)

	// Keys that aren't account-admin editable, and unknown keys: 404.
	assert.Equal(t, http.StatusNotFound, putSetting(admin, 2, "smtp.host", "mail.example.test").StatusCode)
	assert.Equal(t, http.StatusNotFound, putSetting(admin, 2, "quotas.max_upload_size_mb", 10).StatusCode)
	assert.Equal(t, http.StatusNotFound, putSetting(admin, 2, "no.such.key", true).StatusCode)

	// Wrong JSON type, or no value at all: 400.
	assert.Equal(t, http.StatusBadRequest, putSetting(admin, 2, cssSettingKey, "false").StatusCode)
	resp := testutil.MakeRequest(admin, http.MethodPut, "/accounts/2/settings/"+cssSettingKey, testutil.JSONBody(map[string]any{}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// A super-admin may act on any existing account; a missing one is 404.
	ops := f.app(40, 1)
	assert.Equal(t, http.StatusOK, putSetting(ops, 2, cssSettingKey, false).StatusCode)
	assert.Equal(t, http.StatusNotFound, putSetting(ops, 999, cssSettingKey, false).StatusCode)

	var rows int64
	require.NoError(t, f.g.Model(&models.Setting{}).Where("key = ? AND scope_type = 'account' AND scope_id IN (1, 4, 999)", cssSettingKey).Count(&rows).Error)
	assert.Zero(t, rows, "refused writes left no rows")
}

func TestAccountSettings_LockedByParentAndInstance(t *testing.T) {
	f := newAccountSettingsFixture(t)
	admin := f.app(3, 2)

	// The district (parent) turns it off: the school is off and locked.
	f.set(t, settings.ScopeAccount, 4, "false")
	_, item := listSettings(t, admin, 2)
	assert.Equal(t, false, item["effective"])
	assert.Equal(t, true, item["locked"])
	assert.Equal(t, map[string]any{"scope": "account", "account_id": float64(4), "account_name": "Example District"}, item["locked_by"])

	resp := putSetting(admin, 2, cssSettingKey, true)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	body := jsonMap(t, resp)
	assert.Contains(t, fmt.Sprint(body["errors"]), "Turned off by Example District")

	// Off (and inherit) are still allowed under the lock.
	assert.Equal(t, http.StatusOK, putSetting(admin, 2, cssSettingKey, false).StatusCode)
	assert.Equal(t, http.StatusOK, putSetting(admin, 2, cssSettingKey, nil).StatusCode)

	// The instance turns it off too: the instance is the lock now.
	f.set(t, settings.ScopeInstance, 0, "false")
	_, item = listSettings(t, admin, 2)
	assert.Equal(t, map[string]any{"scope": "instance"}, item["locked_by"])
	resp = putSetting(admin, 2, cssSettingKey, true)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Contains(t, fmt.Sprint(jsonMap(t, resp)["errors"]), "Turned off by your Paper administrator")
}

// Course show reports custom_css_enabled with the chain semantics: the
// course's account can't turn hand-written CSS back on under a parent
// that turned it off, and can turn it off under one that allows it.
func TestAccountSettings_CourseJSONFollowsTheChain(t *testing.T) {
	f := newAccountSettingsFixture(t)
	require.NoError(t, f.g.Create(&models.Course{ID: 300, AccountID: 2, Name: "Example Course", CourseCode: "EX", WorkflowState: models.CourseAvailable}).Error)

	// The lookup closure main.go builds.
	lookup := func(ctx context.Context, key string) (string, error) {
		ev, err := f.svc.Get(ctx, key, settings.ScopeHints{AccountID: settingsctx.AccountIDFromContext(ctx)})
		return ev.Value, err
	}
	courseRepo := postgres.NewCourseRepository(f.g)
	enrollRepo := postgres.NewEnrollmentRepository(f.g)
	courses := handlers.NewCourseHandler(
		service.NewCourseService(courseRepo, enrollRepo, postgres.NewSectionRepository(f.g)),
		service.NewEnrollmentService(enrollRepo),
	).SetPageDesigner(postgres.NewAttachmentRepository(f.g), lookup)
	app := testutil.SetupTestApp()
	app.Use(pfStub(3, 2, "TeacherEnrollment"))
	app.Get("/courses/:id", courses.GetCourse)
	policy := service.NewCourseCSSPolicy(courseRepo, lookup)

	enabled := func() any {
		resp := testutil.MakeRequest(app, http.MethodGet, "/courses/300", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return jsonMap(t, resp)["custom_css_enabled"]
	}

	assert.Equal(t, true, enabled(), "nothing set")
	f.set(t, settings.ScopeAccount, 2, "false")
	f.set(t, settings.ScopeAccount, 4, "true")
	assert.Equal(t, false, enabled(), "child off under a parent that allows it")
	assert.False(t, policy.Allowed(context.Background(), 300), "page writes agree")

	f.set(t, settings.ScopeAccount, 2, "true")
	f.set(t, settings.ScopeAccount, 4, "false")
	assert.Equal(t, false, enabled(), "child on under a parent that turned it off")
	assert.False(t, policy.Allowed(context.Background(), 300))

	f.set(t, settings.ScopeAccount, 4, "true")
	assert.Equal(t, true, enabled())
	f.set(t, settings.ScopeInstance, 0, "false")
	assert.Equal(t, false, enabled(), "instance off")
}
