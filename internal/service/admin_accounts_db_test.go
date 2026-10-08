package service_test

// G114 / G115 / G117 / G118 / G119 / G120 — account administration
// services against a real database. All data is made up.

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func strp(s string) *string { return &s }

func TestAdminUsers_CreateUpdateAndLogins(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()
	users := service.NewAdminUserService(g)
	logins := service.NewUserLoginService(g)

	u, err := users.CreateUser(ctx, 2, service.AdminUserInput{Name: strp("Jane Doe"), Email: strp("jane@example.com"), LoginID: strp("jane.doe")})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.AccountID != 2 || !u.RequiresPasswordReset || u.SortableName != "Doe, Jane" {
		t.Fatalf("unexpected new user: %+v", u)
	}
	// Login names are unique across primary and extra logins, any tenant.
	if _, err := users.CreateUser(ctx, 1, service.AdminUserInput{Name: strp("Other"), Email: strp("o@example.com"), LoginID: strp("JANE.DOE")}); !errors.Is(err, service.ErrAdminUserLoginTaken) {
		t.Fatalf("duplicate login accepted: %v", err)
	}
	// Another tenant can't load the user.
	if _, err := users.GetUser(ctx, u.ID, 1); err == nil {
		t.Fatal("GetUser crossed tenants")
	}

	// Admin edit: name, email, SIS id, password.
	if err := users.UpdateUser(ctx, u, service.AdminUserInput{Name: strp("Jane Q Doe"), SISUserID: strp("J-1"), Password: strp("correct-horse-1")}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if u.SortableName != "Doe, Jane Q" || u.SISUserID == nil || *u.SISUserID != "J-1" {
		t.Fatalf("update not applied: %+v", u)
	}

	// Extra local login with its own password.
	l, err := logins.Create(ctx, u.ID, 2, service.UserLoginInput{UniqueID: strp("jdoe-alt"), Password: strp("second-pass-2")})
	if err != nil {
		t.Fatalf("Create login: %v", err)
	}
	if _, err := logins.Create(ctx, u.ID, 1, service.UserLoginInput{UniqueID: strp("x")}); !errors.Is(err, service.ErrUserLoginNotFound) {
		t.Fatalf("login created for another tenant's user: %v", err)
	}
	if _, err := logins.Create(ctx, u.ID, 2, service.UserLoginInput{UniqueID: strp("jane.doe")}); !errors.Is(err, service.ErrUserLoginTaken) {
		t.Fatalf("extra login reused a primary login: %v", err)
	}

	userSvc := service.NewUserService(postgres.NewUserRepository(g)).SetExtraLogins(logins)
	got, err := userSvc.Authenticate(ctx, "JDOE-ALT", "second-pass-2")
	if err != nil || got.ID != u.ID {
		t.Fatalf("sign-in with the extra login failed: %v", err)
	}
	if _, err := userSvc.Authenticate(ctx, "jdoe-alt", "correct-horse-1"); err == nil {
		t.Fatal("extra login with its own password accepted the user's password")
	}
	if _, err := userSvc.Authenticate(ctx, "jane.doe", "correct-horse-1"); err != nil {
		t.Fatalf("primary login broke: %v", err)
	}
	// Timing parity: an unknown name reports found=false (no bcrypt ran,
	// the caller pays the single dummy compare); an existing login with a
	// wrong password reports found=true (its one compare ran).
	if _, found, err := logins.AuthenticateLocal(ctx, "nobody-here", "x"); found || err == nil {
		t.Fatalf("unknown login: found=%v err=%v", found, err)
	}
	if _, found, err := logins.AuthenticateLocal(ctx, "jdoe-alt", "wrong-pass"); !found || err == nil {
		t.Fatalf("wrong password on an existing login: found=%v err=%v", found, err)
	}
	// The email fallback still works for a name that is not a login.
	if _, err := userSvc.Authenticate(ctx, u.Email, "correct-horse-1"); err != nil {
		t.Fatalf("email sign-in broke: %v", err)
	}

	// Suspending the login stops it.
	if _, err := logins.Update(ctx, u.ID, l.ID, 2, service.UserLoginInput{WorkflowState: strp("suspended")}); err != nil {
		t.Fatalf("suspend login: %v", err)
	}
	if _, err := userSvc.Authenticate(ctx, "jdoe-alt", "second-pass-2"); err == nil {
		t.Fatal("suspended login still signs in")
	}

	// An SSO login resolves only through its provider, in its account.
	var provider models.AuthenticationProvider
	if err := g.Exec(`INSERT INTO authentication_providers (account_id, auth_type, workflow_state, position) VALUES (2, 'cas', 'active', 1)`).Error; err != nil {
		t.Skipf("authentication_providers seed unsupported: %v", err)
	}
	g.Where("account_id = ?", 2).First(&provider)
	pid := provider.ID
	if _, err := logins.Create(ctx, u.ID, 2, service.UserLoginInput{UniqueID: strp("jdoe-cas"), AuthenticationProviderID: &pid}); err != nil {
		t.Fatalf("Create SSO login: %v", err)
	}
	found, err := logins.FindForProvider(ctx, pid, 2, "JDOE-CAS")
	if err != nil || found.UserID != u.ID {
		t.Fatalf("FindForProvider: %v", err)
	}
	if _, err := logins.FindForProvider(ctx, pid, 1, "jdoe-cas"); err == nil {
		t.Fatal("FindForProvider crossed tenants")
	}
	if _, err := userSvc.Authenticate(ctx, "jdoe-cas", "correct-horse-1"); err == nil {
		t.Fatal("an SSO login accepted a local password")
	}
}

func readZip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func TestAccountReports_ProvisioningAndCourseState(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()
	sis := newSISService(g)
	runSIS(t, sis, 1, "users", "user_id,login_id,first_name,last_name,email\nR-1,ria,Ria,Reader,ria@example.com\n")
	runSIS(t, sis, 1, "courses", "course_id,short_name,long_name,status\nRC-1,RC1,Reading,active\nRC-2,RC2,Draft,unpublished\n")
	runSIS(t, sis, 1, "enrollments", "course_id,user_id,role,status\nRC-1,R-1,student,active\n")
	runSIS(t, sis, 2, "users", "user_id,login_id,first_name,last_name,email\nZ-1,zed,Zed,Other,zed@example.com\n")

	reports := service.NewAccountReportService(g, nil).SetAsync(false)
	r, err := reports.Start(ctx, 1, 5, service.ReportProvisioning, service.AccountReportParams{Entities: []string{"users", "courses", "enrollments", "bogus"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	full, _ := reports.Get(ctx, r.ID, 1)
	if full.WorkflowState != models.AccountReportComplete || !strings.HasSuffix(full.FileName, ".zip") {
		t.Fatalf("report not complete: %+v", full.WorkflowState)
	}
	files := readZip(t, full.FileData)
	if !strings.Contains(files["users.csv"], "ria") || strings.Contains(files["users.csv"], "zed") {
		t.Fatalf("users.csv leaked across tenants or missed a user:\n%s", files["users.csv"])
	}
	if !strings.Contains(files["enrollments.csv"], "RC-1") {
		t.Fatalf("enrollments.csv missing the enrollment:\n%s", files["enrollments.csv"])
	}
	if _, err := reports.Get(ctx, r.ID, 2); err == nil {
		t.Fatal("another tenant read the report")
	}

	r2, err := reports.Start(ctx, 1, 5, service.ReportUnpublishedCourse, service.AccountReportParams{})
	if err != nil {
		t.Fatal(err)
	}
	r2, _ = reports.Get(ctx, r2.ID, 1)
	csv := string(r2.FileData)
	if !strings.Contains(csv, "RC-2") || strings.Contains(csv, "RC-1") {
		t.Fatalf("unpublished courses report wrong:\n%s", csv)
	}

	if _, err := reports.Start(ctx, 1, 5, "nope", service.AccountReportParams{}); !errors.Is(err, service.ErrUnknownReport) {
		t.Fatalf("unknown report accepted: %v", err)
	}
	foreignTerm := uint(424242)
	if _, err := reports.Start(ctx, 1, 5, service.ReportGradeExport, service.AccountReportParams{EnrollmentTermID: &foreignTerm}); !errors.Is(err, service.ErrReportTermNotFound) {
		t.Fatalf("foreign term accepted: %v", err)
	}
	// Grade export without a calculator still lists the enrollment.
	r3, _ := reports.Start(ctx, 1, 5, service.ReportGradeExport, service.AccountReportParams{})
	r3, _ = reports.Get(ctx, r3.ID, 1)
	if !strings.Contains(string(r3.FileData), "Ria Reader") {
		t.Fatalf("grade export missing the student:\n%s", r3.FileData)
	}
}

func TestAdminTools_RestoreAndQuota(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()

	c := &models.Course{AccountID: 1, Name: "Old Course", CourseCode: "OLD", WorkflowState: models.CourseDeleted}
	g.Create(c)
	tools := service.NewAdminToolsService(g)
	if list, _ := tools.ListDeletedCourses(ctx, 2, "", 10); len(list) != 0 {
		t.Fatal("deleted courses listed for another tenant")
	}
	if _, err := tools.RestoreCourse(ctx, c.ID, 2); err == nil {
		t.Fatal("restored another tenant's course")
	}
	restored, err := tools.RestoreCourse(ctx, c.ID, 1)
	if err != nil || restored.WorkflowState != models.CourseClaimed {
		t.Fatalf("restore: %v %+v", err, restored)
	}

	quotas := service.NewStorageQuotaService(g)
	if err := quotas.SetAccountDefaults(ctx, 1, map[string]int64{"Course": 1}); err != nil {
		t.Fatal(err)
	}
	info, err := quotas.Info(ctx, "Course", c.ID, 1)
	if err != nil || info.QuotaMB != 1 || info.Overridden {
		t.Fatalf("account default not applied: %v %+v", err, info)
	}
	g.Create(&models.Attachment{ContextType: "Course", ContextID: c.ID, UserID: 1, DisplayName: "a.pdf", Filename: "a.pdf", ContentType: "application/pdf", Size: 700 * 1024, StoragePath: "x"})
	if err := quotas.CheckQuota(ctx, "Course", c.ID, 200*1024); err != nil {
		t.Fatalf("upload under quota refused: %v", err)
	}
	if err := quotas.CheckQuota(ctx, "Course", c.ID, 400*1024); !errors.Is(err, service.ErrStorageQuotaExceeded) {
		t.Fatalf("upload over quota accepted: %v", err)
	}
	if _, err := quotas.SetContextQuota(ctx, "Course", c.ID, 2, 5); err == nil {
		t.Fatal("another tenant set the course quota")
	}
	info, err = quotas.SetContextQuota(ctx, "Course", c.ID, 1, 5)
	if err != nil || !info.Overridden || info.QuotaMB != 5 {
		t.Fatalf("override: %v %+v", err, info)
	}
	if err := quotas.CheckQuota(ctx, "Course", c.ID, 400*1024); err != nil {
		t.Fatalf("override not used: %v", err)
	}
	// User contexts (submissions) are never refused.
	if err := quotas.CheckQuota(ctx, "User", 1, 1<<40); err != nil {
		t.Fatalf("user upload refused: %v", err)
	}
}

func TestBrandConfig(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()
	brand := service.NewBrandConfigService(g)

	if _, err := brand.UpdateVariables(ctx, 1, 3, map[string]string{"brand_primary": "red; } body { display:none"}); !errors.Is(err, service.ErrBrandColor) {
		t.Fatalf("CSS injection accepted: %v", err)
	}
	if _, err := brand.UpdateVariables(ctx, 1, 3, map[string]string{"custom_css": "#000000"}); !errors.Is(err, service.ErrBrandVariable) {
		t.Fatalf("unknown variable accepted: %v", err)
	}
	v, err := brand.UpdateVariables(ctx, 1, 3, map[string]string{"brand_primary": "#0A7B3E", "nav_text": ""})
	if err != nil || v.Variables["brand_primary"] != "#0A7B3E" {
		t.Fatalf("UpdateVariables: %v %+v", err, v)
	}
	if other, _ := brand.Get(ctx, 2); len(other.Variables) != 0 {
		t.Fatal("theme leaked to another tenant")
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	if _, err := brand.SetLogo(ctx, 1, 3, svg); !errors.Is(err, service.ErrBrandLogoType) {
		t.Fatalf("SVG logo accepted: %v", err)
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	if v, err = brand.SetLogo(ctx, 1, 3, png); err != nil || !v.HasLogo {
		t.Fatalf("PNG logo: %v", err)
	}
	if _, ct, err := brand.Logo(ctx, 1); err != nil || ct != "image/png" {
		t.Fatalf("Logo: %v %s", err, ct)
	}
}
