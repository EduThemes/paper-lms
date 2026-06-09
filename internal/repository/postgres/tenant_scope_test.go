package postgres_test

// Repository-layer multi-tenancy integration tests. The whole tenant-
// isolation story rests on Read/Delete repo methods filtering by a
// trailing accountID (404, not 403, on a cross-tenant id). The handler
// and service tests assert this with mocks; these assert the actual SQL
// against a migrated Postgres so a missing WHERE clause can't slip
// through. Gated on PARITY_DB_URL / DATABASE_URL via freshDB (user_test.go),
// so they no-op in environments without Postgres.
//
// Two scoping mechanisms are covered:
//   - courses: a direct `account_id = ?` filter
//   - assignments: an indirect `course_id IN (SELECT id FROM courses
//     WHERE account_id = ?)` subquery
// accountID == 0 is the documented auth-internal escape hatch (no scope).

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

func TestCourseRepo_FindByID_TenantScoped(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	for _, id := range []uint{1, 2} {
		if err := g.WithContext(ctx).Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500)
			 ON CONFLICT (id) DO NOTHING`, id, "Acct").Error; err != nil {
			t.Fatalf("seed account %d: %v", id, err)
		}
	}

	courseRepo := postgres.NewCourseRepository(g)
	course := &models.Course{
		AccountID:     1,
		Name:          "Algebra I",
		CourseCode:    "ALG-1",
		WorkflowState: models.CourseAvailable,
	}
	if err := courseRepo.Create(ctx, course); err != nil {
		t.Fatalf("create course: %v", err)
	}

	t.Run("same tenant resolves", func(t *testing.T) {
		got, err := courseRepo.FindByID(ctx, course.ID, 1)
		if err != nil || got == nil {
			t.Fatalf("want course for account 1, got err=%v", err)
		}
	})
	t.Run("cross tenant is denied", func(t *testing.T) {
		got, err := courseRepo.FindByID(ctx, course.ID, 2)
		if err == nil && got != nil {
			t.Fatalf("account 2 must NOT see account 1's course (got id=%d)", got.ID)
		}
	})
	t.Run("accountID 0 escape hatch resolves", func(t *testing.T) {
		got, err := courseRepo.FindByID(ctx, course.ID, 0)
		if err != nil || got == nil {
			t.Fatalf("accountID=0 must bypass scope, got err=%v", err)
		}
	})
}

func TestAssignmentRepo_FindByID_TenantScopedViaCourse(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	for _, id := range []uint{1, 2} {
		if err := g.WithContext(ctx).Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500)
			 ON CONFLICT (id) DO NOTHING`, id, "Acct").Error; err != nil {
			t.Fatalf("seed account %d: %v", id, err)
		}
	}

	courseRepo := postgres.NewCourseRepository(g)
	course := &models.Course{AccountID: 1, Name: "Bio", CourseCode: "BIO-1", WorkflowState: models.CourseAvailable}
	if err := courseRepo.Create(ctx, course); err != nil {
		t.Fatalf("create course: %v", err)
	}

	assignmentRepo := postgres.NewAssignmentRepository(g)
	asg := &models.Assignment{CourseID: course.ID, Name: "Lab 1", WorkflowState: models.AssignmentUnpublished}
	if err := assignmentRepo.Create(ctx, asg); err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	t.Run("same tenant resolves", func(t *testing.T) {
		got, err := assignmentRepo.FindByID(ctx, asg.ID, 1)
		if err != nil || got == nil {
			t.Fatalf("want assignment for account 1, got err=%v", err)
		}
	})
	t.Run("cross tenant is denied", func(t *testing.T) {
		got, err := assignmentRepo.FindByID(ctx, asg.ID, 2)
		if err == nil && got != nil {
			t.Fatalf("account 2 must NOT see account 1's assignment (got id=%d)", got.ID)
		}
	})
	t.Run("accountID 0 escape hatch resolves", func(t *testing.T) {
		got, err := assignmentRepo.FindByID(ctx, asg.ID, 0)
		if err != nil || got == nil {
			t.Fatalf("accountID=0 must bypass scope, got err=%v", err)
		}
	})
}
