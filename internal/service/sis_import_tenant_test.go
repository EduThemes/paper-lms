package service_test

// B065 / B066 regression tests for SISImportService. A CSV uploaded by
// one tenant must stay inside that tenant (new users land in the batch's
// account, SIS-ID lookups never resolve another tenant's rows, courses
// can't be placed in another account), and enrollments.csv re-imports
// must update rows in place and refuse unknown roles.

import (
	"bytes"
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"gorm.io/gorm"
)

func seedSISTenants(t *testing.T, g *gorm.DB) {
	t.Helper()
	for _, id := range []int{1, 2} {
		if err := g.Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500) ON CONFLICT (id) DO NOTHING`,
			id, "Tenant",
		).Error; err != nil {
			t.Fatalf("seed account %d: %v", id, err)
		}
	}
	if err := g.Exec(`SELECT setval(pg_get_serial_sequence('accounts','id'), 10)`).Error; err != nil {
		t.Fatalf("bump account seq: %v", err)
	}
}

func newSISService(g *gorm.DB) *service.SISImportService {
	return service.NewSISImportService(
		postgres.NewSISBatchRepository(g),
		postgres.NewSISBatchErrorRepository(g),
		postgres.NewUserRepository(g),
		postgres.NewCourseRepository(g),
		postgres.NewSectionRepository(g),
		postgres.NewEnrollmentRepository(g),
		g,
	)
}

func runSIS(t *testing.T, svc *service.SISImportService, accountID uint, kind, body string) uint {
	t.Helper()
	ctx := context.Background()
	batch, err := svc.CreateBatch(ctx, accountID)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if err := svc.ProcessImport(ctx, batch.ID, kind, bytes.NewBufferString(body)); err != nil {
		t.Fatalf("ProcessImport(%s): %v", kind, err)
	}
	return batch.ID
}

func TestSISImport_StaysInsideBatchTenant(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()
	svc := newSISService(g)

	// Tenant 1 already owns SIS user "S-100".
	sis := "S-100"
	victim := &models.User{AccountID: 1, Name: "Jane Doe", LoginID: "jane.doe", Email: "jane.doe@example.com", SISUserID: &sis}
	if err := g.Create(victim).Error; err != nil {
		t.Fatalf("seed victim: %v", err)
	}

	// Tenant 2 imports a row reusing that SIS id plus a fresh user.
	batchID := runSIS(t, svc, 2, "users",
		"user_id,login_id,first_name,last_name,email,status\n"+
			"S-100,hijack,Evil,Overwrite,evil@example.com,active\n"+
			"S-200,john.roe,John,Roe,john.roe@example.com,active\n")

	var reloaded models.User
	g.First(&reloaded, victim.ID)
	if reloaded.Name != "Jane Doe" || reloaded.LoginID != "jane.doe" || reloaded.Email != "jane.doe@example.com" {
		t.Fatalf("tenant 1 user was overwritten by tenant 2 import: %+v", reloaded)
	}

	var created models.User
	if err := g.Where("sis_user_id = ?", "S-200").First(&created).Error; err != nil {
		t.Fatalf("new user not created: %v", err)
	}
	if created.AccountID != 2 {
		t.Fatalf("new SIS user landed in account %d, want 2", created.AccountID)
	}

	errs, err := svc.GetBatchErrors(ctx, batchID, 2)
	if err != nil || len(errs) != 1 {
		t.Fatalf("want exactly one row error for the colliding SIS id, got %d (%v)", len(errs), err)
	}

	// Batches are tied to their account: tenant 1 can't read tenant 2's.
	if _, err := svc.GetBatch(ctx, batchID, 1); err == nil {
		t.Fatal("GetBatch returned another tenant's batch")
	}
	if _, err := svc.GetBatchErrors(ctx, batchID, 1); err == nil {
		t.Fatal("GetBatchErrors returned another tenant's batch errors")
	}

	// courses.csv naming another account is refused; without account_id
	// the course lands in the batch account.
	runSIS(t, svc, 2, "courses",
		"course_id,short_name,long_name,account_id,status\n"+
			"C-1,BIO1,Biology,1,active\n"+
			"C-2,CHEM1,Chemistry,,active\n"+
			"C-3,PHYS1,Physics,2,active\n")
	var n int64
	g.Model(&models.Course{}).Where("sis_course_id = ?", "C-1").Count(&n)
	if n != 0 {
		t.Fatal("courses.csv placed a course in another tenant via account_id")
	}
	for _, sisID := range []string{"C-2", "C-3"} {
		var c models.Course
		if err := g.Where("sis_course_id = ?", sisID).First(&c).Error; err != nil {
			t.Fatalf("course %s not created: %v", sisID, err)
		}
		if c.AccountID != 2 {
			t.Fatalf("course %s in account %d, want 2", sisID, c.AccountID)
		}
	}

	// Enrollment rows naming tenant 1's SIS user don't resolve.
	runSIS(t, svc, 2, "enrollments",
		"course_id,user_id,role,status\nC-2,S-100,student,active\n")
	g.Model(&models.Enrollment{}).Where("user_id = ?", victim.ID).Count(&n)
	if n != 0 {
		t.Fatal("tenant 2 enrollments.csv enrolled tenant 1's user")
	}
}

func TestSISImport_EnrollmentReimportAndRoles(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	svc := newSISService(g)

	runSIS(t, svc, 1, "users",
		"user_id,login_id,first_name,last_name,email,status\n"+
			"U-1,student.one,Sam,Student,sam@example.com,active\n")
	runSIS(t, svc, 1, "courses",
		"course_id,short_name,long_name,status\nK-1,ART1,Art,active\n")
	if err := g.Exec(`INSERT INTO custom_roles (name, base_role_type, account_id, workflow_state, permissions)
		VALUES ('Co-Teacher', 'teacher', 1, 'active', '{}')`).Error; err != nil {
		t.Fatalf("seed custom role: %v", err)
	}

	var user models.User
	g.Where("sis_user_id = ?", "U-1").First(&user)
	var course models.Course
	g.Where("sis_course_id = ?", "K-1").First(&course)

	countType := func(typ string) int64 {
		var n int64
		g.Model(&models.Enrollment{}).Where("user_id = ? AND course_id = ? AND type = ?", user.ID, course.ID, typ).Count(&n)
		return n
	}

	// Nightly feed: active, then inactive twice, then completed.
	for _, status := range []string{"active", "inactive", "inactive", "completed"} {
		runSIS(t, svc, 1, "enrollments", "course_id,user_id,role,status\nK-1,U-1,student,"+status+"\n")
	}
	if got := countType("StudentEnrollment"); got != 1 {
		t.Fatalf("re-imports produced %d student enrollment rows, want 1", got)
	}
	var e models.Enrollment
	g.Where("user_id = ? AND course_id = ? AND type = ?", user.ID, course.ID, "StudentEnrollment").First(&e)
	if e.WorkflowState != models.EnrollmentCompleted {
		t.Fatalf("enrollment state %q, want completed", e.WorkflowState)
	}

	// Unknown role is refused, not stored verbatim; capitalised base
	// roles and custom role names resolve to a base Type.
	batchID := runSIS(t, svc, 1, "enrollments",
		"course_id,user_id,role,status\n"+
			"K-1,U-1,Principal,active\n"+
			"K-1,U-1,Teacher,active\n"+
			"K-1,U-1,co-teacher,active\n")
	var bogus int64
	g.Model(&models.Enrollment{}).Where("type NOT IN ?", []string{"StudentEnrollment", "TeacherEnrollment", "TaEnrollment", "ObserverEnrollment", "DesignerEnrollment"}).Count(&bogus)
	if bogus != 0 {
		t.Fatalf("%d enrollment rows with an invalid type", bogus)
	}
	if got := countType("TeacherEnrollment"); got != 1 {
		t.Fatalf("teacher + custom teacher role produced %d TeacherEnrollment rows, want 1", got)
	}
	errs, _ := svc.GetBatchErrors(context.Background(), batchID, 1)
	if len(errs) != 1 {
		t.Fatalf("want 1 row error (invalid role), got %d", len(errs))
	}
}
