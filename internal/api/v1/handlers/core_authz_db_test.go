package handlers_test

// DB-backed regression tests for B069 (account-scoped admin endpoints that
// load rows by bare id) and B172 (masquerade attribution in audit rows).
// The term and audit services take concrete postgres repositories, so these
// run against a scratch, fully migrated database. Skipped when neither
// PARITY_DB_URL nor DATABASE_URL is set. All data is made up.

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func coreScratchDB(t *testing.T) *gorm.DB {
	t.Helper()
	parityURL := os.Getenv("PARITY_DB_URL")
	if parityURL == "" {
		parityURL = os.Getenv("DATABASE_URL")
	}
	if parityURL == "" {
		t.Skip("set PARITY_DB_URL (or DATABASE_URL) to run DB-backed handler tests")
	}
	withDB := func(name string) string {
		u, err := url.Parse(parityURL)
		if err != nil {
			t.Fatalf("parse URL: %v", err)
		}
		u.Path = "/" + name
		return u.String()
	}

	admin, err := sql.Open("postgres", withDB("postgres"))
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	name := fmt.Sprintf("paper_lms_coreauthz_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE %q`, name)); err != nil {
		_ = admin.Close()
		t.Fatalf("create db: %v", err)
	}
	bs, err := sql.Open("postgres", withDB(name))
	if err != nil {
		t.Fatalf("open scratch: %v", err)
	}
	if _, err := bs.Exec(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		_ = bs.Close()
		t.Fatalf("create extension: %v", err)
	}
	_ = bs.Close()

	g, err := gorm.Open(pgdriver.Open(withDB(name)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	if err := db.MigrateUp(g); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	t.Cleanup(func() {
		if raw, err := g.DB(); err == nil {
			_ = raw.Close()
		}
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		_, _ = admin.ExecContext(dropCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		_ = admin.Close()
	})
	for _, id := range []uint{1, 2} {
		if err := g.Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500)
			 ON CONFLICT (id) DO NOTHING`, id, "Acct").Error; err != nil {
			t.Fatalf("seed account %d: %v", id, err)
		}
	}
	// Users referenced by audit rows (audit_logs.user_id has an FK).
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: 3, AccountID: 2, Name: "Admin Example", LoginID: "admin@example.test", Email: "admin@example.test", PasswordHash: "x", Role: "admin"},
		{ID: 8, AccountID: 1, Name: "Jane Doe", LoginID: "jane@example.test", Email: "jane@example.test", PasswordHash: "x", Role: "user"},
	} {
		if err := users.Create(context.Background(), u); err != nil {
			t.Fatalf("seed user %d: %v", u.ID, err)
		}
	}
	return g
}

// B069: an admin of account 2 can't read, update or delete account 1's term
// by putting their own account in the URL and the other tenant's term id.
func TestEnrollmentTerms_CrossTenantTermIsNotFound(t *testing.T) {
	g := coreScratchDB(t)
	ctx := context.Background()
	termRepo := postgres.NewEnrollmentTermRepository(g)
	foreign := &models.EnrollmentTerm{AccountID: 1, Name: "Fall (account 1)", SISTermID: "term-a"}
	if err := termRepo.Create(ctx, foreign); err != nil {
		t.Fatalf("seed term: %v", err)
	}
	own := &models.EnrollmentTerm{AccountID: 2, Name: "Fall (account 2)", SISTermID: "term-b"}
	if err := termRepo.Create(ctx, own); err != nil {
		t.Fatalf("seed term: %v", err)
	}

	h := handlers.NewEnrollmentTermHandler(service.NewEnrollmentTermService(termRepo, g))
	app := testutil.SetupTestApp()
	app.Use(callerStub(3, 2)) // admin of account 2
	app.Get("/accounts/:account_id/terms/:id", h.GetTerm)
	app.Put("/accounts/:account_id/terms/:id", h.UpdateTerm)
	app.Delete("/accounts/:account_id/terms/:id", h.DeleteTerm)

	foreignPath := fmt.Sprintf("/accounts/2/terms/%d", foreign.ID)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, foreignPath, nil).StatusCode)
	body := testutil.JSONBody(map[string]interface{}{"enrollment_term": map[string]interface{}{"name": "Renamed"}})
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodPut, foreignPath, body).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodDelete, foreignPath, nil).StatusCode)

	after, err := termRepo.FindByID(ctx, foreign.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	assert.Equal(t, "Fall (account 1)", after.Name, "foreign term must be untouched")
	assert.NotEqual(t, "deleted", after.WorkflowState, "foreign term must not be deleted")

	assert.Equal(t, http.StatusOK,
		testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/accounts/2/terms/%d", own.ID), nil).StatusCode)
}

// B069: /admin/audit_log/summary counts only the caller's tenant for an
// account admin.
func TestAuditLogSummary_ScopedToCallerTenant(t *testing.T) {
	g := coreScratchDB(t)
	ctx := context.Background()
	auditSvc := service.NewAuditService(postgres.NewAuditLogRepository(g), postgres.NewGradeChangeLogRepository(g), nil)
	one, two := uint(1), uint(2)
	for i := 0; i < 3; i++ {
		if err := auditSvc.LogEvent(ctx, "course_update", 8, nil, &one, "Course", 1, "updated", "{}", "", ""); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := auditSvc.LogEvent(ctx, "course_update", 3, nil, &two, "Course", 2, "updated", "{}", "", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := handlers.NewAuditHandler(auditSvc)
	app := testutil.SetupTestApp()
	app.Use(callerStub(3, 2))
	app.Get("/admin/audit_log/summary", h.GetAuditLogSummary)

	resp := testutil.MakeRequest(app, http.MethodGet, "/admin/audit_log/summary", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	rows, err := testutil.ParseJSONArray(resp)
	assert.NoError(t, err)
	if assert.Len(t, rows, 1) {
		assert.EqualValues(t, 1, rows[0]["count"], "account 2's admin must only see account 2's events")
	}
}

// B172: an audit row written during a masquerade records the real user, and
// a plain-text payload no longer makes the jsonb insert fail silently.
func TestAuditLog_MasqueradeAttributionAndTextPayload(t *testing.T) {
	g := coreScratchDB(t)
	auditRepo := postgres.NewAuditLogRepository(g)
	auditSvc := service.NewAuditService(auditRepo, postgres.NewGradeChangeLogRepository(g), nil)

	// Route the write through a real Fiber request so the masquerade_by
	// Local reaches the service via c.Context().
	app := fiber.New()
	app.Post("/x", func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(8))
		c.Locals("masquerade_by", uint(3))
		acct := uint(1)
		if err := auditSvc.LogEvent(c.Context(), "grade_change", 8, nil, &acct, "Submission", 5, "graded", "POST /api/v1/x", "", ""); err != nil {
			return err
		}
		return c.SendStatus(http.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/x", nil), -1)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("log event through request failed: status=%v err=%v", resp, err)
	}

	var row models.AuditLog
	if err := g.Where("event_type = ?", "grade_change").First(&row).Error; err != nil {
		t.Fatalf("audit row not written: %v", err)
	}
	assert.EqualValues(t, 8, row.UserID)
	if assert.NotNil(t, row.RealUserID, "masquerading admin must be recorded") {
		assert.EqualValues(t, 3, *row.RealUserID)
	}
	assert.Contains(t, row.Payload, "POST /api/v1/x")
}
