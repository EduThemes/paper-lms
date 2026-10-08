package postgres_test

// Integration tests for the communications fixes (announcement visibility,
// inbox recipient search, appointment slot capacity under concurrency).
// Gated on PARITY_DB_URL / DATABASE_URL via freshDB (user_test.go).

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"gorm.io/gorm"
)

func seedCommsAccount(t *testing.T, g *gorm.DB, id uint) {
	t.Helper()
	if err := g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (?, 'Comms Test Tenant', 'active', 'off', 'en', 'higher_ed', 500)
		 ON CONFLICT (id) DO NOTHING`, id).Error; err != nil {
		t.Fatalf("seed account %d: %v", id, err)
	}
}

func seedCommsCourse(t *testing.T, g *gorm.DB, accountID uint, code string) *models.Course {
	t.Helper()
	course := &models.Course{AccountID: accountID, Name: "Course " + code, CourseCode: code, WorkflowState: models.CourseAvailable}
	if err := postgres.NewCourseRepository(g).Create(context.Background(), course); err != nil {
		t.Fatalf("create course: %v", err)
	}
	return course
}

func seedCommsEnrollment(t *testing.T, g *gorm.DB, userID, courseID uint, typ string, sectionID *uint) {
	t.Helper()
	e := &models.Enrollment{UserID: userID, CourseID: courseID, CourseSectionID: sectionID, Type: typ, Role: typ, WorkflowState: "active"}
	if err := g.Create(e).Error; err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}
}

// B151 — audience and section targeting are applied in SQL, so a student
// sees only rows aimed at them and TotalCount / page size are exact; an
// instructor sees every audience plus drafts and delayed posts (B053).
func TestAnnouncementRepo_ListForViewer(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	course := seedCommsCourse(t, g, 1, "ANN-1")
	repo := postgres.NewAnnouncementRepository(g)

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(24 * time.Hour)
	mk := func(title, audience, state string, postedAt, delayed *time.Time) {
		cid := course.ID
		a := &models.Announcement{CourseID: &cid, UserID: 1, Title: title, Message: "m", TargetAudience: audience, WorkflowState: state, PostedAt: postedAt, DelayedPostAt: delayed}
		if err := repo.Create(ctx, a); err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
	}
	mk("all", "all", "published", &past, nil)
	mk("students", "students", "published", &past, nil)
	mk("teachers", "teachers", "published", &past, nil)
	mk("section 5", "section:5", "published", &past, nil)
	mk("section 6", "section:6", "published", &past, nil)
	mk("draft", "all", "draft", nil, nil)
	mk("scheduled", "all", "scheduled", nil, &future)

	student := postgres.AnnouncementViewer{Audiences: []string{"students"}, SectionIDs: []uint{5}}
	res, err := repo.ListForViewer(ctx, course.ID, student, repository.PaginationParams{Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("list student: %v", err)
	}
	if res.TotalCount != 3 {
		t.Fatalf("student TotalCount = %d, want 3 (all, students, section 5)", res.TotalCount)
	}
	if len(res.Items) != 2 {
		t.Fatalf("student page size = %d, want 2", len(res.Items))
	}
	all, _ := repo.ListForViewer(ctx, course.ID, student, repository.PaginationParams{Page: 1, PerPage: 50})
	for _, a := range all.Items {
		switch a.Title {
		case "teachers", "section 6", "draft", "scheduled":
			t.Fatalf("student must not see %q", a.Title)
		}
	}

	teacher, err := repo.ListForViewer(ctx, course.ID, postgres.AnnouncementViewer{Instructor: true}, repository.PaginationParams{Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("list teacher: %v", err)
	}
	if teacher.TotalCount != 7 {
		t.Fatalf("instructor TotalCount = %d, want 7 (every audience, drafts and scheduled)", teacher.TotalCount)
	}
}

// B053 — the delayed-post job publishes due rows, after which students
// see them.
func TestAnnouncementService_PublishScheduled_MakesVisible(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	course := seedCommsCourse(t, g, 1, "ANN-2")
	repo := postgres.NewAnnouncementRepository(g)

	cid := course.ID
	due := time.Now().Add(-time.Minute)
	a := &models.Announcement{CourseID: &cid, UserID: 1, Title: "due", Message: "m", TargetAudience: "all", WorkflowState: "scheduled", DelayedPostAt: &due}
	if err := repo.Create(ctx, a); err != nil {
		t.Fatalf("create: %v", err)
	}

	student := postgres.AnnouncementViewer{Audiences: []string{"students"}}
	before, _ := repo.ListForViewer(ctx, course.ID, student, repository.PaginationParams{Page: 1, PerPage: 10})
	if before.TotalCount != 0 {
		t.Fatalf("scheduled row visible before publishing")
	}

	svc := service.NewAnnouncementService(repo, nil, nil)
	n, err := svc.PublishScheduledAnnouncements(ctx)
	if err != nil || n != 1 {
		t.Fatalf("PublishScheduledAnnouncements = %d, %v; want 1, nil", n, err)
	}
	after, _ := repo.ListForViewer(ctx, course.ID, student, repository.PaginationParams{Page: 1, PerPage: 10})
	if after.TotalCount != 1 {
		t.Fatalf("published row not visible to students (TotalCount=%d)", after.TotalCount)
	}
}

// B059 — recipient search returns course-mates in the caller's tenant
// only; admins may search their whole account.
func TestRecipientSearch_ScopedToSharedCourses(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	seedCommsAccount(t, g, 2)
	seedUserForWallet(t, g, 101, 1, "jane.student@example.com")
	seedUserForWallet(t, g, 102, 1, "john.classmate@example.com")
	seedUserForWallet(t, g, 103, 1, "jim.otherclass@example.com")
	seedUserForWallet(t, g, 104, 2, "jill.othertenant@example.com")
	shared := seedCommsCourse(t, g, 1, "SHARED")
	other := seedCommsCourse(t, g, 1, "OTHER")
	seedCommsEnrollment(t, g, 101, shared.ID, "StudentEnrollment", nil)
	seedCommsEnrollment(t, g, 102, shared.ID, "StudentEnrollment", nil)
	seedCommsEnrollment(t, g, 103, other.ID, "StudentEnrollment", nil)

	rs := postgres.NewRecipientSearchRepository(g)
	got, err := rs.SearchRecipients(ctx, 101, 1, "wallet test", false, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 || got[0].ID != 102 {
		ids := []uint{}
		for _, u := range got {
			ids = append(ids, u.ID)
		}
		t.Fatalf("student search = %v, want only the classmate [102]", ids)
	}

	admin, err := rs.SearchRecipients(ctx, 101, 1, "wallet test", true, 10)
	if err != nil {
		t.Fatalf("admin search: %v", err)
	}
	if len(admin) != 2 {
		t.Fatalf("account-wide search returned %d users, want 2 (102, 103; never the other tenant)", len(admin))
	}
	for _, u := range admin {
		if u.ID == 104 {
			t.Fatalf("cross-tenant user leaked into recipient search")
		}
	}

	// A non-admin cannot search by email address: results never carry
	// emails, so matching on them would let a student confirm who owns
	// an address. Admins still can. (The wallet fixture puts the email in
	// the display name, so give the classmate a plain name first.)
	if err := g.Exec("UPDATE users SET name = ?, sortable_name = ? WHERE id = ?", "John Classmate", "Classmate, John", 102).Error; err != nil {
		t.Fatalf("rename: %v", err)
	}
	byEmail, err := rs.SearchRecipients(ctx, 101, 1, "john.classmate@", false, 10)
	if err != nil {
		t.Fatalf("email search: %v", err)
	}
	if len(byEmail) != 0 {
		t.Fatalf("student email search returned %d users, want 0", len(byEmail))
	}
	adminByEmail, err := rs.SearchRecipients(ctx, 101, 1, "john.classmate@", true, 10)
	if err != nil {
		t.Fatalf("admin email search: %v", err)
	}
	if len(adminByEmail) != 1 || adminByEmail[0].ID != 102 {
		t.Fatalf("admin email search = %v, want [102]", adminByEmail)
	}
}

// B165 — concurrent reservations of the last seat must not overbook.
func TestAppointmentReserve_ConcurrentLastSeat(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	seedCommsAccount(t, g, 1)
	course := seedCommsCourse(t, g, 1, "APPT-1")
	const students = 24
	for i := uint(0); i < students; i++ {
		seedUserForWallet(t, g, 200+i, 1, fmt.Sprintf("appt%d@example.com", i))
	}

	svc := service.NewAppointmentGroupService(
		postgres.NewAppointmentGroupRepository(g),
		postgres.NewAppointmentSlotRepository(g),
		postgres.NewAppointmentReservationRepository(g),
		g,
	)
	start := time.Now().Add(48 * time.Hour)
	group, slots, err := svc.CreateGroup(ctx, &models.AppointmentGroup{
		CourseID: course.ID, Title: "Office hours", CreatedByUserID: 1,
		ParticipantsPerAppointment: 1, MaxAppointmentsPerParticipant: 1, WorkflowState: "active",
	}, []service.SlotInput{{StartAt: start, EndAt: start.Add(15 * time.Minute)}})
	if err != nil || len(slots) != 1 {
		t.Fatalf("create group: %v", err)
	}

	// Warm the connection pool so the callers below don't serialize on
	// connection setup, then release them all at once so their capacity
	// checks overlap.
	if sqlDB, err := g.DB(); err == nil {
		sqlDB.SetMaxIdleConns(students + 4)
		var warm sync.WaitGroup
		for i := 0; i < students; i++ {
			warm.Add(1)
			go func() {
				defer warm.Done()
				_, _ = sqlDB.ExecContext(ctx, "SELECT pg_sleep(0.05)")
			}()
		}
		warm.Wait()
	}
	gate := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, full := 0, 0
	for i := uint(0); i < students; i++ {
		wg.Add(1)
		go func(uid uint) {
			defer wg.Done()
			<-gate
			_, err := svc.Reserve(ctx, group.ID, slots[0].ID, uid)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, service.ErrSlotFull):
				full++
			default:
				t.Errorf("unexpected reserve error: %v", err)
			}
		}(200 + i)
	}
	close(gate)
	wg.Wait()

	if ok != 1 {
		t.Fatalf("%d reservations succeeded for a 1-person slot, want exactly 1", ok)
	}
	var count int64
	g.Model(&models.AppointmentReservation{}).Where("slot_id = ? AND workflow_state = ?", slots[0].ID, "reserved").Count(&count)
	if count != 1 {
		t.Fatalf("slot holds %d reservations, want 1", count)
	}
}
