package postgres_test

// DB-backed regression tests for the core authz lane:
//   - B180: FindByUserAndCourse returns the most privileged active
//     enrollment, not the lowest primary key.
//   - B061: FindByEmailInAccount never matches a user in another tenant.
// Gated on PARITY_DB_URL / DATABASE_URL via freshDB (user_test.go).

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

func seedAccounts(t *testing.T, exec func(sql string, args ...interface{}) error, ids ...uint) {
	t.Helper()
	for _, id := range ids {
		if err := exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500)
			 ON CONFLICT (id) DO NOTHING`, id, "Acct"); err != nil {
			t.Fatalf("seed account %d: %v", id, err)
		}
	}
}

func TestEnrollmentRepo_FindByUserAndCourse_HighestPrivilegeWins(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedAccounts(t, func(q string, a ...interface{}) error { return g.Exec(q, a...).Error }, 1)

	users := postgres.NewUserRepository(g)
	teacher := &models.User{Name: "Pat Teacher", LoginID: "pat@example.test", Email: "pat@example.test", PasswordHash: "x", Role: "user"}
	if err := users.Create(ctx, teacher); err != nil {
		t.Fatalf("create user: %v", err)
	}
	courses := postgres.NewCourseRepository(g)
	course := &models.Course{AccountID: 1, Name: "Biology", CourseCode: "BIO-1", WorkflowState: models.CourseAvailable}
	if err := courses.Create(ctx, course); err != nil {
		t.Fatalf("create course: %v", err)
	}

	enrollments := postgres.NewEnrollmentRepository(g)
	// The observer row (parent pairing) is created FIRST, so it has the
	// lower id — the case that used to shadow the teacher enrollment.
	observer := &models.Enrollment{UserID: teacher.ID, CourseID: course.ID, Type: "ObserverEnrollment", WorkflowState: "active"}
	if err := enrollments.Create(ctx, observer); err != nil {
		t.Fatalf("create observer enrollment: %v", err)
	}
	teach := &models.Enrollment{UserID: teacher.ID, CourseID: course.ID, Type: "TeacherEnrollment", WorkflowState: "active"}
	if err := enrollments.Create(ctx, teach); err != nil {
		t.Fatalf("create teacher enrollment: %v", err)
	}
	if observer.ID >= teach.ID {
		t.Fatalf("fixture expects the observer row to have the lower id (%d vs %d)", observer.ID, teach.ID)
	}

	got, err := enrollments.FindByUserAndCourse(ctx, teacher.ID, course.ID, 1)
	if err != nil {
		t.Fatalf("FindByUserAndCourse: %v", err)
	}
	if got.Type != "TeacherEnrollment" {
		t.Fatalf("want TeacherEnrollment (highest privilege), got %s", got.Type)
	}

	// Cross-tenant scope still applies.
	if _, err := enrollments.FindByUserAndCourse(ctx, teacher.ID, course.ID, 2); err == nil {
		t.Fatal("account 2 must not resolve an enrollment in account 1's course")
	}
}

func TestUserRepo_FindByEmailInAccount_TenantScoped(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedAccounts(t, func(q string, a ...interface{}) error { return g.Exec(q, a...).Error }, 1, 2)

	users := postgres.NewUserRepository(g)
	u := &models.User{AccountID: 1, Name: "Jane Doe", LoginID: "jane@school-a.test", Email: "Jane@School-A.test", PasswordHash: "x", Role: "user"}
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, err := users.FindByEmailInAccount(ctx, "jane@school-a.test", 1)
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("same-tenant lookup (case-insensitive) should resolve user %d, got %v err=%v", u.ID, got, err)
	}
	if got, err := users.FindByEmailInAccount(ctx, "jane@school-a.test", 2); err == nil && got != nil {
		t.Fatalf("account 2 must NOT resolve account 1's user (got id=%d)", got.ID)
	}
	if got, err := users.FindByEmailInAccount(ctx, "jane@school-a.test", 0); err == nil && got != nil {
		t.Fatal("accountID 0 must not act as an unscoped lookup")
	}
}
