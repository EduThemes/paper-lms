package service_test

// G112 — academicSessions become terms, classes land in their term and
// roster enrollments land in the class section. All data is made up.

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestOneRosterSync_TermsAndSections(t *testing.T) {
	svc, g, cleanup := newDeprovisionFixture(t)
	defer cleanup()
	ctx := context.Background()

	if errs := svc.SyncAcademicSessionsForTest(ctx, 1, []service.OneRosterTestSession{
		{SourcedID: "fy27", Status: "active", Title: "2026-2027", Type: "schoolYear", StartDate: "2026-08-15", EndDate: "2027-06-10"},
		{SourcedID: "gp1", Status: "active", Title: "Quarter 1", Type: "gradingPeriod", StartDate: "2026-08-15", EndDate: "2026-10-20"},
	}); len(errs) > 0 {
		t.Fatalf("sync sessions: %v", errs)
	}
	var term models.EnrollmentTerm
	if err := g.Where("account_id = 1 AND sis_term_id = ?", "oneroster:fy27").First(&term).Error; err != nil {
		t.Fatalf("academic session did not become a term: %v", err)
	}
	if term.StartAt == nil || term.Name != "2026-2027" {
		t.Fatalf("term fields not mapped: %+v", term)
	}
	var gp int64
	g.Model(&models.EnrollmentTerm{}).Where("sis_term_id = ?", "oneroster:gp1").Count(&gp)
	if gp != 0 {
		t.Fatal("a grading period became a term")
	}

	if errs := svc.SyncClassInTermForTest(ctx, 1, "c9", "Chemistry", "fy27"); len(errs) > 0 {
		t.Fatalf("sync class: %v", errs)
	}
	var course models.Course
	g.Where("sis_course_id = ?", "oneroster:c9").First(&course)
	if course.EnrollmentTermID == nil || *course.EnrollmentTermID != term.ID {
		t.Fatalf("class not placed in its term: %v", course.EnrollmentTermID)
	}

	if errs := svc.SyncUsersForTest(ctx, 1, []service.OneRosterTestUser{{SourcedID: "s9", Status: "active", Role: "student", GivenName: "Sol", FamilyName: "Student", Username: "sol.student"}}); len(errs) > 0 {
		t.Fatalf("sync users: %v", errs)
	}
	if errs, _ := svc.SyncEnrollmentsForTest(ctx, 1, []service.OneRosterTestEnrollment{{SourcedID: "e9", Status: "active", Role: "student", UserSourcedID: "s9", ClassSourcedID: "c9"}}, false); len(errs) > 0 {
		t.Fatalf("sync enrollments: %v", errs)
	}
	var sec models.CourseSection
	g.Where("course_id = ? AND sis_section_id = ?", course.ID, "oneroster-section:c9").First(&sec)
	var e models.Enrollment
	g.Where("course_id = ? AND type = ?", course.ID, "StudentEnrollment").First(&e)
	if e.CourseSectionID == nil || *e.CourseSectionID != sec.ID {
		t.Fatalf("roster enrollment not in the class section: %v (section %d)", e.CourseSectionID, sec.ID)
	}
}
