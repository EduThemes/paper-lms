package handlers_test

// Review fixes for the grading-policy lane (DB-backed, made-up data):
//   - G019: a student neither lists nor opens an assignment that is only
//     visible to overrides they are not in, and sees their own dates;
//   - G022: a late deduction follows the student's effective due date
//     when an override moves it, and each assignment's missing score is
//     formatted with that assignment's own grading scheme;
//   - G040: the missing-submission job picks up work that comes due late
//     because of an extra-day accommodation.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// assignmentApp mounts the assignment and override routes for a caller,
// wired to the grading-policy service like cmd/server does.
func (f *gpFixture) assignmentApp(userID, accountID uint, enrollmentType string) *fiber.App {
	g := f.g
	assignmentSvc := service.NewAssignmentService(postgres.NewAssignmentRepository(g))
	ah := handlers.NewAssignmentHandler(assignmentSvc)
	ah.SetGradingPolicyService(f.svc)
	enrollmentRepo := postgres.NewEnrollmentRepository(g)
	overrideSvc := service.NewOverrideService(postgres.NewAssignmentOverrideRepository(g), postgres.NewAssignmentOverrideStudentRepository(g),
		enrollmentRepo, postgres.NewSectionRepository(g))
	oh := handlers.NewAssignmentOverrideHandler(overrideSvc, assignmentSvc)
	oh.SetGradingPolicyService(f.svc)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	})
	app.Get("/courses/:course_id/assignments", ah.ListAssignments)
	app.Post("/courses/:course_id/assignments", ah.CreateAssignment)
	app.Put("/courses/:course_id/assignments/:id", ah.UpdateAssignment)
	app.Get("/courses/:course_id/assignments/:id", ah.GetAssignment)
	app.Post("/courses/:course_id/assignments/:assignment_id/overrides", oh.CreateOverride)
	return app
}

func listedAssignment(list []map[string]interface{}, id uint) map[string]interface{} {
	for _, a := range list {
		if uint(a["id"].(float64)) == id {
			return a
		}
	}
	return nil
}

// G019: only_visible_to_overrides hides the assignment from students the
// overrides do not cover; covered students see their override due date.
func TestGradingPolicy_StudentAssignmentVisibility(t *testing.T) {
	f := newGPFixture(t)

	studentA := f.assignmentApp(gpStudentA, 1, "StudentEnrollment")
	resp := testutil.MakeRequest(studentA, http.MethodGet, fmt.Sprintf("/courses/%d/assignments", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	list, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	makeUp := listedAssignment(list, gpOnlyA)
	require.NotNil(t, makeUp, "student A is in the override")
	require.NotNil(t, makeUp["due_at"], "the override's due date")
	due, err := time.Parse(time.RFC3339Nano, makeUp["due_at"].(string))
	require.NoError(t, err)
	assert.True(t, due.After(time.Now().Add(6*24*time.Hour)), "next week's override date, not the assignment's (none)")

	studentB := f.assignmentApp(gpStudentB, 1, "StudentEnrollment")
	resp = testutil.MakeRequest(studentB, http.MethodGet, fmt.Sprintf("/courses/%d/assignments", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	list, err = testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	assert.Nil(t, listedAssignment(list, gpOnlyA), "not assigned to student B")
	assert.NotNil(t, listedAssignment(list, gpA1), "everyone's assignments still listed")

	resp = testutil.MakeRequest(studentB, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", gpCourse, gpOnlyA), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(studentA, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", gpCourse, gpOnlyA), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Staff see every assignment.
	teacher := f.assignmentApp(gpTeacher, 1, "TeacherEnrollment")
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d", gpCourse, gpOnlyA), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// G022: work graded late takes the late deduction; once an override moves
// the student's due date past the submission, the deduction is removed.
func TestGradingPolicy_OverrideRemovesLateDeduction(t *testing.T) {
	f := newGPFixture(t)
	ctx := context.Background()
	require.NoError(t, f.g.Create(&models.LatePolicy{CourseID: gpCourse, LateSubmissionDeductionEnabled: true,
		LateSubmissionDeduction: 10, LateSubmissionInterval: "day"}).Error)

	// Due 12 hours ago, handed in now: one (daily) interval late.
	halfDayAgo := time.Now().Add(-12 * time.Hour)
	require.NoError(t, f.g.Model(&models.Assignment{}).Where("id = ?", gpPastDue).Update("due_at", halfDayAgo).Error)
	require.NoError(t, f.sub.Create(ctx, &models.Submission{AssignmentID: gpPastDue, UserID: gpStudentB,
		SubmissionType: strp("online_text_entry"), Body: strp("lab notes")}))
	graded, err := f.sub.Grade(ctx, gpPastDue, gpStudentB, gpTeacher, 1, "10")
	require.NoError(t, err)
	require.True(t, graded.Late)
	require.NotNil(t, graded.Score)
	assert.InDelta(t, 9.0, *graded.Score, 0.001, "10% off for one day late")

	nextWeek := time.Now().Add(7 * 24 * time.Hour)
	teacher := f.assignmentApp(gpTeacher, 1, "TeacherEnrollment")
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/overrides", gpCourse, gpPastDue),
		testutil.JSONBody(map[string]interface{}{"assignment_override": map[string]interface{}{
			"title": "Extension", "due_at": nextWeek.Format(time.RFC3339), "student_ids": []uint{gpStudentB},
		}}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var sub models.Submission
	require.NoError(t, f.g.Where("assignment_id = ? AND user_id = ?", gpPastDue, gpStudentB).First(&sub).Error)
	assert.False(t, sub.Late, "no longer late against the extended date")
	require.NotNil(t, sub.Score)
	assert.InDelta(t, 10.0, *sub.Score, 0.001, "the late deduction is gone")
	assert.Nil(t, sub.PointsDeducted)
	require.NotNil(t, sub.Grade)
	assert.Equal(t, "10", *sub.Grade)
}

// G040: an assignment whose base due date is long past still comes due
// for a student with a 10-day extension; the hourly job marks that work
// missing and applies the missing policy.
func TestGradingPolicy_MissingJobCoversAccommodationExtensions(t *testing.T) {
	f := newGPFixture(t)
	ctx := context.Background()
	require.NoError(t, f.g.Create(&models.LatePolicy{CourseID: gpCourse, MissingSubmissionDeductionEnabled: true, MissingSubmissionDeduction: 50}).Error)
	baseDue := time.Now().Add(-10*24*time.Hour - time.Hour)
	essay := &models.Assignment{ID: 820, CourseID: gpCourse, Name: "Essay", PointsPossible: gpf(10), GradingType: "points",
		SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic", DueAt: &baseDue}
	require.NoError(t, f.g.Create(essay).Error)
	days := 10
	require.NoError(t, f.g.Create(&models.StudentAccommodation{UserID: gpStudentA, CourseID: ptrUint(gpCourse), AccommodationType: "extended_time",
		ExtraDays: &days, Status: "active", CreatedByID: gpTeacher, EffectiveFrom: time.Now().Add(-30 * 24 * time.Hour)}).Error)

	_, err := f.svc.ApplyMissingPolicy(ctx, 72*time.Hour)
	require.NoError(t, err)

	var sub models.Submission
	require.NoError(t, f.g.Where("assignment_id = ? AND user_id = ?", essay.ID, gpStudentA).First(&sub).Error,
		"the extended student's missing row was created")
	assert.True(t, sub.Missing)
	require.NotNil(t, sub.Score)
	assert.InDelta(t, 5.0, *sub.Score, 0.001)
}

// G022 / G039: the missing policy formats each assignment's score with
// that assignment's scheme, not the first one seen in the course.
func TestGradingPolicy_MissingScoreUsesEachAssignmentsScheme(t *testing.T) {
	f := newGPFixture(t)
	ctx := context.Background()
	require.NoError(t, f.g.Create(&models.LatePolicy{CourseID: gpCourse, MissingSubmissionDeductionEnabled: true, MissingSubmissionDeduction: 50}).Error)
	effort := &models.GradingStandard{ContextType: "Account", ContextID: 1, Title: "Effort", Data: `[["Strong",0.4],["Weak",0]]`, WorkflowState: "active"}
	require.NoError(t, f.g.Create(effort).Error)
	yesterday := time.Now().Add(-24 * time.Hour)
	ownScheme := &models.Assignment{ID: 830, CourseID: gpCourse, Name: "Reflection", PointsPossible: gpf(10), GradingType: "letter_grade",
		SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic",
		DueAt: &yesterday, Position: -2, GradingStandardID: &effort.ID}
	courseScheme := &models.Assignment{ID: 831, CourseID: gpCourse, Name: "Summary", PointsPossible: gpf(10), GradingType: "letter_grade",
		SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic",
		DueAt: &yesterday, Position: -1}
	require.NoError(t, f.g.Create(ownScheme).Error)
	require.NoError(t, f.g.Create(courseScheme).Error)

	_, err := f.svc.ApplyCourseMissingPolicy(ctx, mustCourse(t, f.g, gpCourse))
	require.NoError(t, err)

	grade := func(aid uint) string {
		var s models.Submission
		require.NoError(t, f.g.Where("assignment_id = ? AND user_id = ?", aid, gpStudentA).First(&s).Error)
		require.NotNil(t, s.Grade)
		return *s.Grade
	}
	assert.Equal(t, "Strong", grade(ownScheme.ID), "50% on the assignment's own scheme")
	assert.Equal(t, "F", grade(courseScheme.ID), "50% on the default scale")
}

// G018: anonymous assignments always post manually, on create and when
// anonymous grading is turned on later.
func TestGradingPolicy_AnonymousAssignmentsPostManually(t *testing.T) {
	f := newGPFixture(t)
	teacher := f.assignmentApp(gpTeacher, 1, "TeacherEnrollment")
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/assignments", gpCourse),
		testutil.JSONBody(map[string]interface{}{"assignment": map[string]interface{}{
			"name": "Blind review", "points_possible": 10, "anonymous_grading": true, "post_policy": "automatic",
		}}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "manual", body["post_policy"])

	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d", gpCourse, gpA1),
		testutil.JSONBody(map[string]interface{}{"assignment": map[string]interface{}{"anonymous_grading": true}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var a models.Assignment
	require.NoError(t, f.g.First(&a, gpA1).Error)
	assert.Equal(t, "manual", a.PostPolicy)
}

// G025: with "Display totals for All Grading Periods" off, a student gets
// per-period grades but no all-periods total; staff still see it.
func TestGradingPolicy_DisplayTotalsForAllPeriods(t *testing.T) {
	f := newGPFixture(t)
	group := &models.GradingPeriodGroup{AccountID: 1, Title: "Semesters", WorkflowState: "active"}
	require.NoError(t, f.g.Create(group).Error)
	require.NoError(t, f.g.Create(&models.GradingPeriod{GradingPeriodGroupID: group.ID, Title: "Fall",
		StartDate: time.Now().Add(-60 * 24 * time.Hour), EndDate: time.Now().Add(60 * 24 * time.Hour), WorkflowState: "active"}).Error)
	term := &models.EnrollmentTerm{AccountID: 1, Name: "2026", GradingPeriodGroupID: &group.ID, WorkflowState: "active"}
	require.NoError(t, f.g.Create(term).Error)
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", gpCourse).Update("enrollment_term_id", term.ID).Error)

	student := f.app(gpStudentA, 1, "StudentEnrollment")
	resp := testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	me := gpStudent(t, body, gpStudentA)
	assert.Nil(t, me["current_score"], "no all-periods total for the student")
	periods, _ := me["grading_periods"].([]interface{})
	require.Len(t, periods, 1)
	assert.NotNil(t, periods[0].(map[string]interface{})["current_score"], "the period grade is shown")

	teacher := f.app(gpTeacher, 1, "TeacherEnrollment")
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err = testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.NotNil(t, gpStudent(t, body, gpStudentA)["current_score"], "staff keep the total")

	// Turning the option on gives the student the total too.
	require.NoError(t, f.g.Model(&models.GradingPeriodGroup{}).Where("id = ?", group.ID).Update("display_totals", true).Error)
	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	body, err = testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.NotNil(t, gpStudent(t, body, gpStudentA)["current_score"])
}
