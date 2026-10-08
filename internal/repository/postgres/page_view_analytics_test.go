package postgres_test

// Integration tests for the course-analytics page view aggregations (G154).
// Gated on PARITY_DB_URL / DATABASE_URL via freshDB (user_test.go).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"gorm.io/gorm"
)

func TestPageViewRepo_CourseAnalyticsAggregates(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	seedCommsAccount(t, g, 1)
	seedUserForWallet(t, g, 101, 1, "student-a@example.test")
	seedUserForWallet(t, g, 102, 1, "student-b@example.test")
	seedUserForWallet(t, g, 103, 1, "teacher@example.test")
	course := seedCommsCourse(t, g, 1, "AN-1")
	other := seedCommsCourse(t, g, 1, "AN-2")
	seedCommsEnrollment(t, g, 101, course.ID, "StudentEnrollment", nil)
	seedCommsEnrollment(t, g, 102, course.ID, "StudentEnrollment", nil)
	seedCommsEnrollment(t, g, 103, course.ID, "TeacherEnrollment", nil)
	seedCommsEnrollment(t, g, 101, other.ID, "StudentEnrollment", nil)

	repo := postgres.NewPageViewRepository(g)
	day1 := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)
	mk := func(user, courseID uint, at time.Time, participated bool, category string, seconds int) *models.PageView {
		pv := &models.PageView{UserID: user, ContextType: "Course", ContextID: courseID, URL: "/courses/x", Participated: participated, AssetCategory: category, InteractionSeconds: seconds, CreatedAt: at}
		if err := repo.Create(ctx, pv); err != nil {
			t.Fatalf("create page view: %v", err)
		}
		return pv
	}
	mk(101, course.ID, day1, false, "assignments", 30)
	mk(101, course.ID, day1, false, "pages", 20)
	mk(101, course.ID, day2, true, "discussions", 0)
	mk(102, course.ID, day2, false, "assignments", 10)
	mk(103, course.ID, day2, false, "assignments", 50) // teacher: not counted
	mk(101, other.ID, day2, false, "assignments", 40)  // other course: not counted

	days, err := repo.ActivityByDay(ctx, course.ID, 0, nil)
	if err != nil {
		t.Fatalf("activity by day: %v", err)
	}
	want := []repository.PageViewDayCount{
		{Date: "2026-03-02", Views: 2, Participations: 0},
		{Date: "2026-03-03", Views: 1, Participations: 1},
	}
	if len(days) != len(want) {
		t.Fatalf("days = %+v, want %+v", days, want)
	}
	for i := range want {
		if days[i] != want[i] {
			t.Fatalf("day %d = %+v, want %+v", i, days[i], want[i])
		}
	}

	since := day2
	only, err := repo.ActivityByDay(ctx, course.ID, 101, &since)
	if err != nil || len(only) != 1 || only[0].Participations != 1 || only[0].Views != 0 {
		t.Fatalf("student 101 since day2 = %+v, %v", only, err)
	}

	totals, err := repo.TotalsByUser(ctx, course.ID)
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if _, ok := totals[103]; ok {
		t.Fatalf("teacher must not appear in student totals: %+v", totals)
	}
	a := totals[101]
	if a.Views != 2 || a.Participations != 1 || a.InteractionSeconds != 50 || a.LastViewedAt == nil {
		t.Fatalf("student 101 totals = %+v", a)
	}

	cats, err := repo.CategoryCounts(ctx, course.ID, 0)
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	if len(cats) == 0 || cats[0].Category != "assignments" || cats[0].Views != 2 {
		t.Fatalf("categories = %+v", cats)
	}

	list, err := repo.ListByUserAndContext(ctx, 101, "Course", course.ID, repository.PaginationParams{Page: 1, PerPage: 2})
	if err != nil || list.TotalCount != 3 || len(list.Items) != 2 || !list.Items[0].CreatedAt.Equal(day2) {
		t.Fatalf("list = %+v, %v", list, err)
	}
}

func TestPageViewRepo_UpdateInteractionOwnerOnlyAndTouchActivity(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()

	seedCommsAccount(t, g, 1)
	seedUserForWallet(t, g, 101, 1, "student-a@example.test")
	course := seedCommsCourse(t, g, 1, "AN-3")
	seedCommsEnrollment(t, g, 101, course.ID, "StudentEnrollment", nil)

	repo := postgres.NewPageViewRepository(g)
	pv := &models.PageView{UserID: 101, ContextType: "Course", ContextID: course.ID, URL: "/courses/x", InteractionSeconds: 40}
	if err := repo.Create(ctx, pv); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.UpdateInteraction(ctx, pv.ID, 999, 100); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("another user's page view must be not found, got %v", err)
	}
	if err := repo.UpdateInteraction(ctx, pv.ID, 101, 25); err != nil {
		t.Fatalf("update: %v", err)
	}
	var got models.PageView
	g.First(&got, pv.ID)
	if got.InteractionSeconds != 40 {
		t.Fatalf("interaction must never go down, got %d", got.InteractionSeconds)
	}
	if err := repo.UpdateInteraction(ctx, pv.ID, 101, 90); err != nil {
		t.Fatalf("update: %v", err)
	}
	g.First(&got, pv.ID)
	if got.InteractionSeconds != 90 {
		t.Fatalf("interaction = %d, want 90", got.InteractionSeconds)
	}

	if err := repo.TouchEnrollmentActivity(ctx, 101, course.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}
	var e models.Enrollment
	g.Where("user_id = ? AND course_id = ?", 101, course.ID).First(&e)
	if e.LastActivityAt == nil {
		t.Fatalf("last_activity_at not stamped")
	}
}
