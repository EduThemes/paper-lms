package postgres_test

// Content-lane repository regressions, run against a migrated Postgres
// (freshDB in user_test.go; skipped without PARITY_DB_URL/DATABASE_URL).

import (
	"context"
	"fmt"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"gorm.io/gorm"
)

func seedLaneCourse(t *testing.T, g *gorm.DB, accountID uint) *models.Course {
	t.Helper()
	ctx := context.Background()
	if err := g.WithContext(ctx).Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500) ON CONFLICT (id) DO NOTHING`,
		accountID, fmt.Sprintf("District %d", accountID)).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}
	c := &models.Course{AccountID: accountID, Name: "Course", CourseCode: fmt.Sprintf("C-%d", accountID), WorkflowState: models.CourseAvailable}
	if err := postgres.NewCourseRepository(g).Create(ctx, c); err != nil {
		t.Fatalf("seed course: %v", err)
	}
	return c
}

// B044: two pages with the same (or same-slugging) title get distinct
// URLs, so each is reachable and edits land on the right one.
func TestPageRepo_Create_UniqueSlugPerCourse(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	course := seedLaneCourse(t, g, 1)
	other := seedLaneCourse(t, g, 1)
	repo := postgres.NewPageRepository(g)

	mk := func(courseID uint, title string) *models.WikiPage {
		p := &models.WikiPage{CourseID: courseID, Title: title, WorkflowState: "active"}
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("create %q: %v", title, err)
		}
		return p
	}
	a := mk(course.ID, "Week 1")
	b := mk(course.ID, "Week 1")
	c := mk(course.ID, "Week 1!")
	d := mk(other.ID, "Week 1")

	if a.URL != "week-1" || b.URL != "week-1-2" || c.URL != "week-1-3" {
		t.Fatalf("want week-1, week-1-2, week-1-3; got %q %q %q", a.URL, b.URL, c.URL)
	}
	if d.URL != "week-1" {
		t.Fatalf("slugs are per course; got %q", d.URL)
	}
	got, err := repo.FindByCourseAndURL(ctx, course.ID, b.URL)
	if err != nil || got.ID != b.ID {
		t.Fatalf("second page must be reachable by its own slug (got %+v, err %v)", got, err)
	}

	// A deleted page frees its slug.
	if err := repo.Delete(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	e := mk(course.ID, "Week 1")
	if e.URL != "week-1" {
		t.Fatalf("deleted page's slug should be reusable, got %q", e.URL)
	}
}

// B135: a removed (soft-deleted) blueprint association can be re-added.
func TestBlueprintSubscriptionRepo_ReAddAfterRemove(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := postgres.NewBlueprintSubscriptionRepository(g)

	sub := &models.BlueprintSubscription{BlueprintTemplateID: 3, ChildCourseID: 20, WorkflowState: "active"}
	if err := repo.Create(ctx, sub); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete(ctx, sub.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	again := &models.BlueprintSubscription{BlueprintTemplateID: 3, ChildCourseID: 20, WorkflowState: "active"}
	if err := repo.Create(ctx, again); err != nil {
		t.Fatalf("re-add must not hit idx_template_child: %v", err)
	}
	if again.ID != sub.ID {
		t.Fatalf("re-add should revive row %d, got id %d", sub.ID, again.ID)
	}
	got, err := repo.FindByTemplateAndChild(ctx, 3, 20)
	if err != nil || got.WorkflowState != "active" {
		t.Fatalf("revived subscription must be active (got %+v, err %v)", got, err)
	}
}

// B139: a deleted attachment is not found (so it is not downloadable).
func TestAttachmentRepo_FindByID_ExcludesDeleted(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	course := seedLaneCourse(t, g, 1)
	repo := postgres.NewAttachmentRepository(g)

	att := &models.Attachment{
		ContextType: "Course", ContextID: course.ID, UserID: 1,
		DisplayName: "handout.pdf", Filename: "handout.pdf", ContentType: "application/pdf",
		Size: 10, StoragePath: "Course/1/x/handout.pdf", WorkflowState: "active",
		FileState: "available", UploadStatus: "success",
	}
	if err := repo.Create(ctx, att); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.FindByID(ctx, att.ID, 1); err != nil {
		t.Fatalf("live attachment must resolve: %v", err)
	}
	if err := repo.Delete(ctx, att.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got, err := repo.FindByID(ctx, att.ID, 1); err == nil {
		t.Fatalf("deleted attachment must not resolve (got %+v)", got)
	}
	if got, err := repo.FindByID(ctx, att.ID, 0); err == nil {
		t.Fatalf("deleted attachment must not resolve unscoped either (got %+v)", got)
	}
}

// B143: pace items come back in course order (module position, then
// item position), not module_item_id order.
func TestCoursePaceModuleItemRepo_ListByPaceID_CourseOrder(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	course := seedLaneCourse(t, g, 1)
	modules := postgres.NewModuleRepository(g)
	items := postgres.NewModuleItemRepository(g)

	m1 := &models.ContextModule{CourseID: course.ID, Name: "Module 1", Position: 1, WorkflowState: "active"}
	m5 := &models.ContextModule{CourseID: course.ID, Name: "Module 5", Position: 5, WorkflowState: "active"}
	for _, m := range []*models.ContextModule{m1, m5} {
		if err := modules.Create(ctx, m); err != nil {
			t.Fatalf("module: %v", err)
		}
	}
	mkItem := func(moduleID uint, title string, pos int) *models.ContentTag {
		it := &models.ContentTag{ContextModuleID: moduleID, ContentType: "ContextModuleSubHeader", Title: title, Position: pos, WorkflowState: "active"}
		if err := items.Create(ctx, it); err != nil {
			t.Fatalf("item: %v", err)
		}
		return it
	}
	// Created in this order → ids ascend: m5 item first, Module 1 item
	// added later (higher id), plus a reordered pair inside Module 1.
	late5 := mkItem(m5.ID, "Module 5 work", 1)
	m1second := mkItem(m1.ID, "Module 1 second", 2)
	m1first := mkItem(m1.ID, "Module 1 first", 1)

	pace := &models.CoursePace{CourseID: course.ID, WorkflowState: "unpublished"}
	if err := postgres.NewCoursePaceRepository(g).Create(ctx, pace); err != nil {
		t.Fatalf("pace: %v", err)
	}
	paceItems := postgres.NewCoursePaceModuleItemRepository(g)
	if err := paceItems.BulkUpsert(ctx, []models.CoursePaceModuleItem{
		{CoursePaceID: pace.ID, ModuleItemID: late5.ID, Duration: 1},
		{CoursePaceID: pace.ID, ModuleItemID: m1second.ID, Duration: 1},
		{CoursePaceID: pace.ID, ModuleItemID: m1first.ID, Duration: 1},
	}); err != nil {
		t.Fatalf("pace items: %v", err)
	}

	got, err := paceItems.ListByPaceID(ctx, pace.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []uint{m1first.ID, m1second.ID, late5.ID}
	if len(got) != len(want) {
		t.Fatalf("want %d items, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i].ModuleItemID != want[i] {
			t.Fatalf("position %d: want module item %d, got %d (full: %+v)", i, want[i], got[i].ModuleItemID, got)
		}
	}
}
