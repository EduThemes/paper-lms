package service_test

// DB-backed tests for the student Learning Mastery view (G152) and its
// agreement with the instructor Learning Mastery Gradebook (G153).
// Skipped without PARITY_DB_URL / DATABASE_URL (freshDB).

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

type masteryFixture struct {
	svc      *service.MasteryGradebookService
	courseID uint
	student  uint
	other    uint
	teacher  uint
	highest  uint // outcome using "highest"
	latest   uint // outcome using "latest"
}

func seedMasteryFixture(t *testing.T, g *gorm.DB) masteryFixture {
	t.Helper()
	ctx := context.Background()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if err := g.Exec(q, args...).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	exec(`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		VALUES (1, 'Mastery Tenant', 'active', 'off', 'en', 'higher_ed', 500)`)
	for _, u := range []struct {
		id   uint
		name string
	}{{201, "Jane Doe"}, {202, "John Roe"}, {203, "Pat Teacher"}} {
		exec(`INSERT INTO users (id, account_id, name, email, login_id, password_hash, role) VALUES (?, 1, ?, ?, ?, 'x', 'user')`,
			u.id, u.name, u.name+"@example.test", u.name+"@example.test")
	}
	course := &models.Course{AccountID: 1, Name: "Biology", CourseCode: "BIO-1", WorkflowState: models.CourseAvailable}
	if err := postgres.NewCourseRepository(g).Create(ctx, course); err != nil {
		t.Fatalf("course: %v", err)
	}
	for _, e := range []struct {
		user uint
		typ  string
	}{{201, "StudentEnrollment"}, {202, "StudentEnrollment"}, {203, "TeacherEnrollment"}} {
		if err := g.Create(&models.Enrollment{UserID: e.user, CourseID: course.ID, Type: e.typ, Role: e.typ, WorkflowState: "active"}).Error; err != nil {
			t.Fatalf("enrollment: %v", err)
		}
	}

	groupRepo := postgres.NewLearningOutcomeGroupRepository(g)
	group := &models.LearningOutcomeGroup{ContextType: "Course", ContextID: course.ID, Title: "Lab skills", WorkflowState: "active"}
	if err := groupRepo.Create(ctx, group); err != nil {
		t.Fatalf("group: %v", err)
	}
	outcomeRepo := postgres.NewLearningOutcomeRepository(g)
	resultRepo := postgres.NewLearningOutcomeResultRepository(g)
	outcomeSvc := service.NewLearningOutcomeService(groupRepo, outcomeRepo, resultRepo)
	mk := func(title, method string) uint {
		// No ratings given: the service supplies Canvas's default scale
		// (ratings_data is jsonb, so "" used to fail the insert).
		o := &models.LearningOutcome{ContextType: "Course", ContextID: course.ID, OutcomeGroupID: group.ID, Title: title,
			CalculationMethod: method, MasteryPoints: 3}
		if err := outcomeSvc.CreateOutcome(ctx, o); err != nil {
			t.Fatalf("outcome: %v", err)
		}
		if o.PointsPossible != 5 {
			t.Fatalf("points_possible from default ratings = %v", o.PointsPossible)
		}
		return o.ID
	}
	highest := mk("Measures accurately", "highest")
	latest := mk("Records data", "latest")

	older := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 2, 8, 10, 0, 0, 0, time.UTC)
	add := func(user, outcome, asset uint, score float64, at time.Time) {
		s, p := score, 10.0
		r := &models.LearningOutcomeResult{UserID: user, LearningOutcomeID: outcome, ContextType: "Course", ContextID: course.ID,
			AssociatedAssetType: "Assignment", AssociatedAssetID: asset, Score: &s, Possible: &p, SubmittedAt: &at, AssessedAt: &at, Title: "Lab report"}
		if _, err := resultRepo.Upsert(ctx, r); err != nil {
			t.Fatalf("result: %v", err)
		}
	}
	// Student 201: 9/10 then 4/10 on both outcomes.
	add(201, highest, 1, 9, older)
	add(201, highest, 2, 4, newer)
	add(201, latest, 1, 9, older)
	add(201, latest, 2, 4, newer)
	// Classmate 202 must never leak into 201's view.
	add(202, highest, 1, 10, newer)

	svc := service.NewMasteryGradebookService(
		postgres.NewEnrollmentRepository(g), outcomeRepo, resultRepo, postgres.NewUserRepository(g),
		service.NewOutcomeProficiencyService(postgres.NewOutcomeProficiencyRepository(g)))
	svc.SetOutcomeGroupRepo(groupRepo)
	return masteryFixture{svc: svc, courseID: course.ID, student: 201, other: 202, teacher: 203, highest: highest, latest: latest}
}

func TestGetStudentMastery_HonorsCalculationMethodAndMatchesGradebook(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	f := seedMasteryFixture(t, g)
	ctx := context.Background()

	view, err := f.svc.GetStudentMastery(ctx, f.courseID, f.student, 1)
	if err != nil {
		t.Fatalf("student mastery: %v", err)
	}
	if view.UserName != "Jane Doe" || len(view.Outcomes) != 2 {
		t.Fatalf("view = %+v", view)
	}
	byID := map[uint]service.StudentMasteryOutcome{}
	for _, o := range view.Outcomes {
		byID[o.ID] = o
	}
	hi, lt := byID[f.highest], byID[f.latest]
	if hi.Score == nil || lt.Score == nil || hi.Possible == nil {
		t.Fatalf("scores missing: %+v %+v", hi, lt)
	}
	// Default proficiency scale tops out at its highest rating; 90% vs 40%.
	if !(*hi.Score > *lt.Score) {
		t.Fatalf("highest (%v) must beat latest (%v) for 9 then 4", *hi.Score, *lt.Score)
	}
	if !hi.Mastered || lt.Mastered {
		t.Fatalf("mastered: highest=%v latest=%v", hi.Mastered, lt.Mastered)
	}
	if hi.GroupTitle != "Lab skills" {
		t.Fatalf("group title = %q", hi.GroupTitle)
	}
	if len(hi.Results) != 2 || hi.Results[0].RawScore == nil || *hi.Results[0].RawScore != 4 {
		t.Fatalf("results newest first, own only: %+v", hi.Results)
	}
	if view.Mastered != 1 || view.Assessed != 2 {
		t.Fatalf("counts mastered=%d assessed=%d", view.Mastered, view.Assessed)
	}

	// G153: the instructor's gradebook cell is the same number.
	gb, err := f.svc.GetMasteryGradebook(ctx, f.courseID, 1)
	if err != nil {
		t.Fatalf("gradebook: %v", err)
	}
	for _, cell := range gb.Cells {
		if cell.UserID != f.student {
			continue
		}
		want := byID[cell.OutcomeID].Score
		if cell.Score == nil || want == nil || *cell.Score != *want {
			t.Fatalf("gradebook cell %d = %v, student view = %v", cell.OutcomeID, cell.Score, want)
		}
	}
}

func TestGetStudentMastery_NonStudentNotFound(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	f := seedMasteryFixture(t, g)
	ctx := context.Background()

	if _, err := f.svc.GetStudentMastery(ctx, f.courseID, f.teacher, 1); !errors.Is(err, service.ErrMasteryStudentNotFound) {
		t.Fatalf("teacher: err = %v", err)
	}
	if _, err := f.svc.GetStudentMastery(ctx, f.courseID, f.student, 999); !errors.Is(err, service.ErrMasteryStudentNotFound) {
		t.Fatalf("other tenant: err = %v", err)
	}
	if f.svc.ObservesStudent(ctx, f.other, f.student, f.courseID, 1) {
		t.Fatalf("a classmate is not an observer")
	}
}
