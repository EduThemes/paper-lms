package service_test

// B074 / B174 regression tests for the OneRoster sync (real Postgres via
// the PARITY_DB_URL pattern; newDeprovisionFixture lives in
// oneroster_deprovision_test.go).

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestOneRosterSync_AdministratorIsNotTenantAdminAndRolesSurvive(t *testing.T) {
	svc, g, cleanup := newDeprovisionFixture(t)
	defer cleanup()
	ctx := context.Background()

	rows := []service.OneRosterTestUser{
		{SourcedID: "a1", Status: "active", Role: "administrator", GivenName: "Pat", FamilyName: "Principal", Username: "pat.principal"},
		{SourcedID: "t1", Status: "active", Role: "teacher", GivenName: "Terry", FamilyName: "Teacher", Username: "terry.teacher"},
	}
	if errs := svc.SyncUsersForTest(ctx, 1, rows); len(errs) > 0 {
		t.Fatalf("sync users: %v", errs)
	}

	var principal, teacher models.User
	g.Where("sis_user_id = ?", "oneroster:a1").First(&principal)
	g.Where("sis_user_id = ?", "oneroster:t1").First(&teacher)
	if principal.Role != "user" {
		t.Fatalf("SIS administrator provisioned with role %q, want user", principal.Role)
	}

	// A Paper admin promotes the teacher; the next sync must not demote.
	if err := g.Model(&models.User{}).Where("id = ?", teacher.ID).Update("role", "admin").Error; err != nil {
		t.Fatal(err)
	}
	if errs := svc.SyncUsersForTest(ctx, 1, rows); len(errs) > 0 {
		t.Fatalf("second sync: %v", errs)
	}
	g.First(&teacher, teacher.ID)
	if teacher.Role != "admin" {
		t.Fatalf("sync overwrote a Paper-managed role: got %q, want admin", teacher.Role)
	}
}

func TestOneRosterSync_DropsRemovedEnrollmentsAndLinksGuardians(t *testing.T) {
	svc, g, cleanup := newDeprovisionFixture(t)
	defer cleanup()
	ctx := context.Background()

	users := []service.OneRosterTestUser{
		{SourcedID: "s1", Status: "active", Role: "student", GivenName: "Sam", FamilyName: "Student", Username: "sam.student", Agents: []string{"g1"}},
		{SourcedID: "s2", Status: "active", Role: "student", GivenName: "Sky", FamilyName: "Student", Username: "sky.student"},
		{SourcedID: "g1", Status: "active", Role: "guardian", GivenName: "Gale", FamilyName: "Guardian", Username: "gale.guardian", Agents: []string{"s1"}},
	}
	if errs := svc.SyncUsersForTest(ctx, 1, users); len(errs) > 0 {
		t.Fatalf("sync users: %v", errs)
	}
	if errs := svc.SyncClassesForTest(ctx, 1, []service.OneRosterTestClass{{"p1", "Period 1"}, {"p2", "Period 2"}}); len(errs) > 0 {
		t.Fatalf("sync classes: %v", errs)
	}

	userID := func(sourced string) uint {
		var u models.User
		g.Where("sis_user_id = ?", "oneroster:"+sourced).First(&u)
		return u.ID
	}
	courseID := func(sourced string) uint {
		var c models.Course
		g.Where("sis_course_id = ?", "oneroster:"+sourced).First(&c)
		return c.ID
	}
	state := func(user, course uint, typ string) models.EnrollmentWorkflow {
		var e models.Enrollment
		if err := g.Where("user_id = ? AND course_id = ? AND type = ?", user, course, typ).
			Order("(workflow_state = 'deleted') ASC, id ASC").First(&e).Error; err != nil {
			return ""
		}
		return e.WorkflowState
	}
	s1, s2, g1 := userID("s1"), userID("s2"), userID("g1")
	p1, p2 := courseID("p1"), courseID("p2")

	// Initial full roster: s1 and s2 in period 1, guardian of s1 too.
	initial := []service.OneRosterTestEnrollment{
		{SourcedID: "e1", Status: "active", Role: "student", UserSourcedID: "s1", ClassSourcedID: "p1"},
		{SourcedID: "e2", Status: "active", Role: "student", UserSourcedID: "s2", ClassSourcedID: "p1"},
		{SourcedID: "e3", Status: "active", Role: "guardian", UserSourcedID: "g1", ClassSourcedID: "p1"},
	}
	if errs, reason := svc.SyncEnrollmentsForTest(ctx, 1, initial, true); len(errs) > 0 || reason != "" {
		t.Fatalf("initial enrollment sync: %v %s", errs, reason)
	}
	var obs models.Enrollment
	if err := g.Where("user_id = ? AND course_id = ? AND type = ?", g1, p1, "ObserverEnrollment").First(&obs).Error; err != nil {
		t.Fatalf("guardian has no observer enrollment: %v", err)
	}
	if obs.AssociatedUserID == nil || *obs.AssociatedUserID != s1 {
		t.Fatalf("guardian observer enrollment not associated with the child: %+v", obs.AssociatedUserID)
	}

	// s1 switches to period 2: the SIS marks e1 tobedeleted.
	moved := []service.OneRosterTestEnrollment{
		{SourcedID: "e1", Status: "tobedeleted", Role: "student", UserSourcedID: "s1", ClassSourcedID: "p1"},
		{SourcedID: "e4", Status: "active", Role: "student", UserSourcedID: "s1", ClassSourcedID: "p2"},
		{SourcedID: "e2", Status: "active", Role: "student", UserSourcedID: "s2", ClassSourcedID: "p1"},
	}
	if errs, reason := svc.SyncEnrollmentsForTest(ctx, 1, moved, true); len(errs) > 0 || reason != "" {
		t.Fatalf("move sync: %v %s", errs, reason)
	}
	if got := state(s1, p1, "StudentEnrollment"); got != models.EnrollmentDeleted {
		t.Fatalf("tobedeleted enrollment state %q, want deleted", got)
	}
	if got := state(s1, p2, "StudentEnrollment"); got != models.EnrollmentActive {
		t.Fatalf("new period enrollment state %q, want active", got)
	}
	// The guardian follows the child.
	if got := state(g1, p1, "ObserverEnrollment"); got != models.EnrollmentDeleted {
		t.Fatalf("guardian kept access to the dropped class: %q", got)
	}
	if got := state(g1, p2, "ObserverEnrollment"); got != models.EnrollmentActive {
		t.Fatalf("guardian did not follow the child to the new class: %q", got)
	}

	// s2 is simply omitted from the next full roster: dropped.
	omitted := []service.OneRosterTestEnrollment{
		{SourcedID: "e4", Status: "active", Role: "student", UserSourcedID: "s1", ClassSourcedID: "p2"},
	}
	if errs, reason := svc.SyncEnrollmentsForTest(ctx, 1, omitted, true); len(errs) > 0 || reason != "" {
		t.Fatalf("omission sync: %v %s", errs, reason)
	}
	if got := state(s2, p1, "StudentEnrollment"); got != models.EnrollmentDeleted {
		t.Fatalf("enrollment missing from the full roster state %q, want deleted", got)
	}

	// Re-adding s2 reactivates the same row rather than duplicating it.
	readd := append(omitted, service.OneRosterTestEnrollment{SourcedID: "e2", Status: "active", Role: "student", UserSourcedID: "s2", ClassSourcedID: "p1"})
	if errs, _ := svc.SyncEnrollmentsForTest(ctx, 1, readd, true); len(errs) > 0 {
		t.Fatalf("re-add sync: %v", errs)
	}
	var n int64
	g.Model(&models.Enrollment{}).Where("user_id = ? AND course_id = ? AND type = ?", s2, p1, "StudentEnrollment").Count(&n)
	if n != 1 || state(s2, p1, "StudentEnrollment") != models.EnrollmentActive {
		t.Fatalf("re-add produced %d rows / state %q, want 1 active", n, state(s2, p1, "StudentEnrollment"))
	}

	// An empty enrollment roster trips the circuit breaker.
	if _, reason := svc.SyncEnrollmentsForTest(ctx, 1, nil, true); reason == "" {
		t.Fatal("empty enrollment roster did not abort the absence pass")
	}
	if state(s1, p2, "StudentEnrollment") != models.EnrollmentActive {
		t.Fatal("aborted absence pass still removed enrollments")
	}
}
