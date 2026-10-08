package postgres_test

// Grading-lane parity regressions that need real SQL (post/hide predicate,
// override-student uniqueness and the 000072 migration). Gated on
// PARITY_DB_URL / DATABASE_URL via freshDB. All data is made up.

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

type gradingSeed struct {
	courseID     uint
	assignmentID uint
	otherAsgID   uint
	userIDs      []uint
}

func seedGrading(t *testing.T, g *gorm.DB) gradingSeed {
	t.Helper()
	ctx := context.Background()
	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (1, 'Root', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`,
	).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}
	course := &models.Course{AccountID: 1, Name: "Bio", CourseCode: "BIO-1", WorkflowState: models.CourseAvailable}
	if err := postgres.NewCourseRepository(g).Create(ctx, course); err != nil {
		t.Fatalf("create course: %v", err)
	}
	asgRepo := postgres.NewAssignmentRepository(g)
	a1 := &models.Assignment{CourseID: course.ID, Name: "Lab 1", WorkflowState: models.AssignmentPublished, Published: true, PostPolicy: "manual"}
	a2 := &models.Assignment{CourseID: course.ID, Name: "Lab 2", WorkflowState: models.AssignmentPublished, Published: true}
	for _, a := range []*models.Assignment{a1, a2} {
		if err := asgRepo.Create(ctx, a); err != nil {
			t.Fatalf("create assignment: %v", err)
		}
	}
	seed := gradingSeed{courseID: course.ID, assignmentID: a1.ID, otherAsgID: a2.ID}
	for i, email := range []string{"jane@example.test", "john@example.test", "sam@example.test", "alex@example.test"} {
		u := &models.User{Name: "Student " + string(rune('A'+i)), LoginID: email, Email: email, PasswordHash: "x", Role: "user"}
		if err := g.WithContext(ctx).Create(u).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
		seed.userIDs = append(seed.userIDs, u.ID)
	}
	return seed
}

// B012/B088: posting releases excused submissions too (they carry no
// score), keeps already-posted timestamps, and hiding clears every posted
// row — including one posted by an instructor comment without a grade.
func TestGradingParity_PostHideGradesPredicate(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seed := seedGrading(t, g)
	repo := postgres.NewSubmissionRepository(g)

	earlier := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	score := 7.0
	rows := []*models.Submission{
		{AssignmentID: seed.assignmentID, UserID: seed.userIDs[0], Score: &score, WorkflowState: models.SubmissionGraded},
		{AssignmentID: seed.assignmentID, UserID: seed.userIDs[1], Excused: true, WorkflowState: models.SubmissionUnsubmitted},
		{AssignmentID: seed.assignmentID, UserID: seed.userIDs[2], WorkflowState: models.SubmissionSubmitted},
		{AssignmentID: seed.assignmentID, UserID: seed.userIDs[3], Score: &score, PostedAt: &earlier, WorkflowState: models.SubmissionGraded},
	}
	for _, r := range rows {
		if err := repo.Create(ctx, r); err != nil {
			t.Fatalf("create submission: %v", err)
		}
	}

	now := time.Now().Truncate(time.Second)
	if err := repo.PostGradesByAssignment(ctx, seed.assignmentID, &now); err != nil {
		t.Fatalf("post: %v", err)
	}
	get := func(uid uint) *models.Submission {
		s, err := repo.FindByAssignmentAndUser(ctx, seed.assignmentID, uid, 1)
		if err != nil {
			t.Fatalf("find: %v", err)
		}
		return s
	}
	if get(seed.userIDs[0]).PostedAt == nil {
		t.Errorf("graded submission must be posted")
	}
	if get(seed.userIDs[1]).PostedAt == nil {
		t.Errorf("excused submission must be posted")
	}
	if get(seed.userIDs[2]).PostedAt != nil {
		t.Errorf("ungraded submission must stay unposted")
	}
	if p := get(seed.userIDs[3]).PostedAt; p == nil || !p.Equal(earlier) {
		t.Errorf("already-posted submission must keep its posted_at, got %v want %v", p, earlier)
	}

	// An instructor comment posts an ungraded submission; hiding must
	// hide it too.
	commented := get(seed.userIDs[2])
	commented.PostedAt = &now
	if err := repo.Update(ctx, commented); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := repo.PostGradesByAssignment(ctx, seed.assignmentID, nil); err != nil {
		t.Fatalf("hide: %v", err)
	}
	for _, uid := range seed.userIDs {
		if get(uid).PostedAt != nil {
			t.Errorf("user %d: hide must clear posted_at", uid)
		}
	}
}

// B112: a student can be in overrides on several assignments; deleting an
// override frees the student's slot on that assignment.
func TestGradingParity_OverrideStudentSlots(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seed := seedGrading(t, g)
	svc := service.NewOverrideService(postgres.NewAssignmentOverrideRepository(g), postgres.NewAssignmentOverrideStudentRepository(g), nil, nil)
	student := seed.userIDs[0]

	o1 := &models.AssignmentOverride{AssignmentID: seed.assignmentID, Title: "Extension"}
	o2 := &models.AssignmentOverride{AssignmentID: seed.otherAsgID, Title: "Extension"}
	for _, o := range []*models.AssignmentOverride{o1, o2} {
		if err := svc.CreateOverride(ctx, o); err != nil {
			t.Fatalf("create override: %v", err)
		}
	}
	if err := svc.AddStudent(ctx, o1.ID, student, seed.assignmentID); err != nil {
		t.Fatalf("add to first assignment: %v", err)
	}
	if err := svc.AddStudent(ctx, o2.ID, student, seed.otherAsgID); err != nil {
		t.Fatalf("same student on a second assignment must be allowed: %v", err)
	}

	o3 := &models.AssignmentOverride{AssignmentID: seed.assignmentID, Title: "Second"}
	if err := svc.CreateOverride(ctx, o3); err != nil {
		t.Fatalf("create override: %v", err)
	}
	if err := svc.AddStudent(ctx, o3.ID, student, seed.assignmentID); err == nil {
		t.Fatalf("a second override for the same student and assignment must conflict")
	}
	if err := svc.DeleteOverride(ctx, o1.ID); err != nil {
		t.Fatalf("delete override: %v", err)
	}
	if err := svc.AddStudent(ctx, o3.ID, student, seed.assignmentID); err != nil {
		t.Fatalf("after deleting the override the student must be addable again: %v", err)
	}

	// A legacy orphan (override soft-deleted without cleaning its rows)
	// does not block a new override either.
	if err := g.Exec(`UPDATE assignment_overrides SET workflow_state = 'deleted' WHERE id = ?`, o2.ID).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	o4 := &models.AssignmentOverride{AssignmentID: seed.otherAsgID, Title: "Third"}
	if err := svc.CreateOverride(ctx, o4); err != nil {
		t.Fatalf("create override: %v", err)
	}
	if err := svc.AddStudent(ctx, o4.ID, student, seed.otherAsgID); err != nil {
		t.Fatalf("orphan row from a deleted override must not block: %v", err)
	}
}

// 000072 down must apply even after students joined overrides on several
// assignments (the restored UNIQUE(user_id) would otherwise fail), and up
// must re-apply cleanly.
func TestGradingParity_Migration000072Reversible(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seed := seedGrading(t, g)
	svc := service.NewOverrideService(postgres.NewAssignmentOverrideRepository(g), postgres.NewAssignmentOverrideStudentRepository(g), nil, nil)
	for _, asg := range []uint{seed.assignmentID, seed.otherAsgID} {
		o := &models.AssignmentOverride{AssignmentID: asg, Title: "Extension"}
		if err := svc.CreateOverride(ctx, o); err != nil {
			t.Fatalf("create override: %v", err)
		}
		if err := svc.AddStudent(ctx, o.ID, seed.userIDs[0], asg); err != nil {
			t.Fatalf("add student: %v", err)
		}
	}

	for _, f := range []string{
		"../../db/migrations/000072_override_student_unique_per_assignment.down.sql",
		"../../db/migrations/000072_override_student_unique_per_assignment.up.sql",
	} {
		sqlText, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if err := g.Exec(string(sqlText)).Error; err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
}
