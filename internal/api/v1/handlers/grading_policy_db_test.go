package handlers_test

// DB-backed tests for the grading-policy lane (G018 post policies, G019
// overrides, G020 drop rules, G021 excused, G022 late / missing + missing
// policy, G023 grading types, G025 grading periods, G028 history, G029
// final grade override, G039 schemes, G040 accommodations). All data is
// made up.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const (
	gpTeacher   = uint(40)
	gpStudentA  = uint(41)
	gpStudentB  = uint(42)
	gpOutsider  = uint(43)
	gpTeacherB  = uint(44)
	gpCourse    = uint(300)
	gpCourseB   = uint(301)
	gpSectionA  = uint(601)
	gpSectionB  = uint(602)
	gpGroup     = uint(700)
	gpA1        = uint(801)
	gpA2        = uint(802)
	gpA3        = uint(803)
	gpOnlyA     = uint(804)
	gpPastDue   = uint(805)
	gpLetter    = uint(806)
	gpOtherTask = uint(900)
)

type gpFixture struct {
	g   *gorm.DB
	svc *service.GradingPolicyService
	sub *service.SubmissionService
}

func gpf(v float64) *float64 { return &v }

func newGPFixture(t *testing.T) *gpFixture {
	t.Helper()
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: gpTeacher, AccountID: 1, Name: "Terry Teacher", LoginID: "terry.gp@example.test", Email: "terry.gp@example.test", PasswordHash: "x", Role: "user"},
		{ID: gpStudentA, AccountID: 1, Name: "Avery Student", LoginID: "avery@example.test", Email: "avery@example.test", PasswordHash: "x", Role: "user"},
		{ID: gpStudentB, AccountID: 1, Name: "Blake Student", LoginID: "blake@example.test", Email: "blake@example.test", PasswordHash: "x", Role: "user"},
		{ID: gpOutsider, AccountID: 1, Name: "Olive Outsider", LoginID: "olive@example.test", Email: "olive@example.test", PasswordHash: "x", Role: "user"},
		{ID: gpTeacherB, AccountID: 2, Name: "Tara Teacher", LoginID: "tara.gp@example.test", Email: "tara.gp@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	require.NoError(t, g.Create(&models.Course{ID: gpCourse, AccountID: 1, Name: "Algebra", CourseCode: "ALG", WorkflowState: models.CourseAvailable}).Error)
	require.NoError(t, g.Create(&models.Course{ID: gpCourseB, AccountID: 2, Name: "Poetry", CourseCode: "POE", WorkflowState: models.CourseAvailable}).Error)
	require.NoError(t, g.Create(&models.CourseSection{ID: gpSectionA, CourseID: gpCourse, Name: "Period 1", WorkflowState: "active"}).Error)
	require.NoError(t, g.Create(&models.CourseSection{ID: gpSectionB, CourseID: gpCourse, Name: "Period 2", WorkflowState: "active"}).Error)
	secA, secB := gpSectionA, gpSectionB
	for _, e := range []*models.Enrollment{
		{UserID: gpTeacher, CourseID: gpCourse, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: gpStudentA, CourseID: gpCourse, CourseSectionID: &secA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: gpStudentB, CourseID: gpCourse, CourseSectionID: &secB, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: gpTeacherB, CourseID: gpCourseB, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
	}
	require.NoError(t, g.Create(&models.AssignmentGroup{ID: gpGroup, CourseID: gpCourse, Name: "Homework", Rules: `{"drop_lowest":1}`, WorkflowState: "available"}).Error)
	group := gpGroup
	yesterday := time.Now().Add(-24 * time.Hour)
	nextWeek := time.Now().Add(7 * 24 * time.Hour)
	for _, a := range []*models.Assignment{
		{ID: gpA1, CourseID: gpCourse, AssignmentGroupID: &group, Name: "HW 1", PointsPossible: gpf(10), GradingType: "points", SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic"},
		{ID: gpA2, CourseID: gpCourse, AssignmentGroupID: &group, Name: "HW 2", PointsPossible: gpf(10), GradingType: "points", SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic"},
		{ID: gpA3, CourseID: gpCourse, AssignmentGroupID: &group, Name: "HW 3", PointsPossible: gpf(10), GradingType: "points", SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "manual"},
		{ID: gpOnlyA, CourseID: gpCourse, AssignmentGroupID: &group, Name: "Make-up", PointsPossible: gpf(10), GradingType: "points", SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic", OnlyVisibleToOverrides: true},
		{ID: gpPastDue, CourseID: gpCourse, Name: "Lab report", PointsPossible: gpf(10), GradingType: "points", SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic", DueAt: &yesterday},
		{ID: gpLetter, CourseID: gpCourse, Name: "Presentation", PointsPossible: gpf(20), GradingType: "letter_grade", SubmissionTypes: "on_paper", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic", DueAt: &nextWeek},
		{ID: gpOtherTask, CourseID: gpCourseB, Name: "Sonnet", PointsPossible: gpf(10), GradingType: "points", SubmissionTypes: "online_text_entry", Published: true, WorkflowState: models.AssignmentPublished, PostPolicy: "automatic"},
	} {
		require.NoError(t, g.Create(a).Error)
	}
	// The make-up assignment is assigned to student A only.
	ov := &models.AssignmentOverride{AssignmentID: gpOnlyA, Title: "Avery", WorkflowState: "active", DueAt: &nextWeek}
	require.NoError(t, g.Create(ov).Error)
	require.NoError(t, g.Create(&models.AssignmentOverrideStudent{AssignmentOverrideID: ov.ID, UserID: gpStudentA, AssignmentID: gpOnlyA}).Error)

	now := time.Now()
	grade := func(aid, uid uint, score float64, posted bool) {
		s := &models.Submission{AssignmentID: aid, UserID: uid, Score: gpf(score), EnteredScore: gpf(score), WorkflowState: models.SubmissionGraded, Attempt: 1, SubmittedAt: &now}
		if posted {
			s.PostedAt = &now
		}
		require.NoError(t, g.Create(s).Error)
	}
	// Student A: 10, 4, 8 (unposted), make-up 9. Student B: HW1 excused, HW2 6.
	grade(gpA1, gpStudentA, 10, true)
	grade(gpA2, gpStudentA, 4, true)
	grade(gpA3, gpStudentA, 8, false)
	grade(gpOnlyA, gpStudentA, 9, true)
	require.NoError(t, g.Create(&models.Submission{AssignmentID: gpA1, UserID: gpStudentB, Excused: true, WorkflowState: models.SubmissionGraded, PostedAt: &now}).Error)
	grade(gpA2, gpStudentB, 6, true)

	submissionRepo := postgres.NewSubmissionRepository(g)
	assignmentRepo := postgres.NewAssignmentRepository(g)
	enrollmentRepo := postgres.NewEnrollmentRepository(g)
	courseRepo := postgres.NewCourseRepository(g)
	standards := postgres.NewGradingStandardRepository(g)
	subSvc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo,
		postgres.NewLatePolicyRepository(g), courseRepo, postgres.NewGradingPeriodGroupRepository(g),
		postgres.NewGradingPeriodRepository(g), postgres.NewGroupMembershipRepository(g))
	overrideSvc := service.NewOverrideService(postgres.NewAssignmentOverrideRepository(g), postgres.NewAssignmentOverrideStudentRepository(g), enrollmentRepo, postgres.NewSectionRepository(g))
	subSvc.SetOverrideService(overrideSvc)
	policyRepo := postgres.NewGradingPolicyRepository(g)
	policy := service.NewGradingPolicyService(policyRepo, standards, subSvc)
	subSvc.SetGradingStandardRepository(standards)
	subSvc.SetGradingPolicyRepository(policyRepo)
	subSvc.SetPeriodResolver(policy)
	return &gpFixture{g: g, svc: policy, sub: subSvc}
}

// app mounts the grading-policy routes for a caller.
func (f *gpFixture) app(userID, accountID uint, enrollmentType string) *fiber.App {
	g := f.g
	submissionRepo := postgres.NewSubmissionRepository(g)
	assignmentRepo := postgres.NewAssignmentRepository(g)
	standards := postgres.NewGradingStandardRepository(g)
	h := handlers.NewGradingPolicyHandler(f.svc, postgres.NewCourseRepository(g), assignmentRepo, submissionRepo, standards, nil)
	sh := handlers.NewSubmissionHandler(f.sub, postgres.NewSubmissionCommentRepository(g), postgres.NewAttachmentRepository(g),
		postgres.NewUserRepository(g), assignmentRepo, nil, nil, nil, nil, nil)
	sh.SetGradingPolicyService(f.svc)
	gs := handlers.NewGradingStandardHandler(standards)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	})
	app.Get("/courses/:course_id/grades", h.CourseGrades)
	app.Get("/courses/:course_id/assignment_dates", h.MyAssignmentDates)
	app.Get("/courses/:course_id/effective_due_dates", h.EffectiveDueDates)
	app.Put("/courses/:course_id/students/:user_id/score_override", h.SetScoreOverride)
	app.Put("/courses/:course_id/post_policy", h.SetPostPolicy)
	app.Get("/courses/:course_id/grading_schemes", h.ListCourseSchemes)
	app.Put("/courses/:course_id/grading_scheme", h.SetCourseScheme)
	app.Put("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/late_policy_status", h.SetLateStatus)
	app.Post("/courses/:course_id/late_policy/apply", h.ApplyLatePolicy)
	app.Get("/courses/:course_id/assignments/:assignment_id/submissions/:user_id/history", h.SubmissionHistory)
	app.Post("/courses/:course_id/assignments/:id/post_grades", sh.PostGrades)
	app.Post("/courses/:course_id/assignments/:id/hide_grades", sh.HideGrades)
	app.Post("/accounts/:account_id/grading_standards", gs.CreateAccountGradingStandard)
	app.Get("/accounts/:account_id/grading_standards", gs.ListAccountGradingStandards)
	return app
}

func gpStudent(t *testing.T, body map[string]interface{}, uid uint) map[string]interface{} {
	t.Helper()
	students, _ := body["students"].([]interface{})
	for _, s := range students {
		m := s.(map[string]interface{})
		if uint(m["user_id"].(float64)) == uid {
			return m
		}
	}
	t.Fatalf("student %d not in grades", uid)
	return nil
}

// G020 / G021 / G019 / G018: the server total drops the lowest homework,
// leaves excused and unassigned work out, and the student view only
// counts posted grades.
func TestGradingPolicy_CourseGrades(t *testing.T) {
	f := newGPFixture(t)
	teacher := f.app(gpTeacher, 1, "TeacherEnrollment")
	resp := testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)

	// A: 10, 4, 8, 9 in Homework, drop lowest (4) -> 27/30 = 90%.
	a := gpStudent(t, body, gpStudentA)
	assert.InDelta(t, 90.0, a["current_score"].(float64), 0.01)
	// B: HW1 excused, make-up unassigned, HW2 6/10 is the only graded
	// work and the single drop never drops the last item -> 60%.
	b := gpStudent(t, body, gpStudentB)
	assert.InDelta(t, 60.0, b["current_score"].(float64), 0.01)
	assert.Contains(t, b["unassigned_assignment_ids"], float64(gpOnlyA))

	// Student A sees their own grade without the unposted HW 3:
	// 10, 4, 9 drop 4 -> 19/20 = 95%.
	student := f.app(gpStudentA, 1, "StudentEnrollment")
	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err = testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Len(t, body["students"], 1)
	assert.InDelta(t, 95.0, gpStudent(t, body, gpStudentA)["current_score"].(float64), 0.01)

	// ...and cannot read a classmate's grades.
	resp = testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/grades?user_id=%d", gpCourse, gpStudentB), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Another tenant's teacher gets a 404 for the course.
	foreign := f.app(gpTeacherB, 2, "TeacherEnrollment")
	resp = testutil.MakeRequest(foreign, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// G019: effective dates come from the student's override; an
// only-visible-to-overrides assignment is absent for others.
func TestGradingPolicy_AssignmentDates(t *testing.T) {
	f := newGPFixture(t)
	resp := testutil.MakeRequest(f.app(gpStudentB, 1, "StudentEnrollment"), http.MethodGet, fmt.Sprintf("/courses/%d/assignment_dates", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	dates := body["assignments"].(map[string]interface{})
	assert.NotContains(t, dates, fmt.Sprint(gpOnlyA))
	assert.Contains(t, dates, fmt.Sprint(gpA1))

	resp = testutil.MakeRequest(f.app(gpStudentA, 1, "StudentEnrollment"), http.MethodGet, fmt.Sprintf("/courses/%d/assignment_dates", gpCourse), nil)
	body, err = testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	dates = body["assignments"].(map[string]interface{})
	require.Contains(t, dates, fmt.Sprint(gpOnlyA))
	assert.NotZero(t, dates[fmt.Sprint(gpOnlyA)].(map[string]interface{})["override_id"])

	// Submitting to an assignment not assigned to you is refused.
	err = f.sub.Create(context.Background(), &models.Submission{AssignmentID: gpOnlyA, UserID: gpStudentB, SubmissionType: strp("online_text_entry"), Body: strp("my work")})
	assert.Error(t, err)
}

func strp(s string) *string { return &s }

// G029: a final grade override is stored per student, shown with the
// grades, and refused for non-students and other tenants.
func TestGradingPolicy_ScoreOverride(t *testing.T) {
	f := newGPFixture(t)
	teacher := f.app(gpTeacher, 1, "TeacherEnrollment")
	url := fmt.Sprintf("/courses/%d/students/%d/score_override", gpCourse, gpStudentA)
	resp := testutil.MakeRequest(teacher, http.MethodPut, url, testutil.JSONBody(map[string]interface{}{"override_grade": "B"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "B", body["override_grade"])

	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	body, _ = testutil.ParseJSONMap(resp)
	a := gpStudent(t, body, gpStudentA)
	require.NotNil(t, a["override_score"])
	assert.Equal(t, "B", a["override_grade"])

	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/%d/students/%d/score_override", gpCourse, gpOutsider),
		testutil.JSONBody(map[string]interface{}{"override_score": 80}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, url, testutil.JSONBody(map[string]interface{}{"override_grade": "Q+"}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	foreign := f.app(gpTeacherB, 2, "TeacherEnrollment")
	resp = testutil.MakeRequest(foreign, http.MethodPut, url, testutil.JSONBody(map[string]interface{}{"override_score": 10}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Clearing removes it.
	resp = testutil.MakeRequest(teacher, http.MethodPut, url, testutil.JSONBody(map[string]interface{}{"override_score": nil}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var n int64
	f.g.Model(&models.CourseScoreOverride{}).Where("user_id = ?", gpStudentA).Count(&n)
	assert.Zero(t, n)
}

// G018: graded-only posting by section posts only that section's graded
// work; the course post policy flips its assignments to manual.
func TestGradingPolicy_PostBySection(t *testing.T) {
	f := newGPFixture(t)
	now := time.Now()
	require.NoError(t, f.g.Create(&models.Submission{AssignmentID: gpA3, UserID: gpStudentB, Score: gpf(7), WorkflowState: models.SubmissionGraded, SubmittedAt: &now}).Error)
	teacher := f.app(gpTeacher, 1, "TeacherEnrollment")
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/post_grades", gpCourse, gpA3),
		testutil.JSONBody(map[string]interface{}{"graded_only": true, "section_ids": []uint{gpSectionB}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var subs []models.Submission
	require.NoError(t, f.g.Where("assignment_id = ?", gpA3).Find(&subs).Error)
	for _, s := range subs {
		if s.UserID == gpStudentB {
			assert.NotNil(t, s.PostedAt, "section B posted")
		} else {
			assert.Nil(t, s.PostedAt, "section A still hidden")
		}
	}
	// Foreign assignment through this course is a 404.
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/assignments/%d/post_grades", gpCourse, gpOtherTask), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/%d/post_policy", gpCourse), testutil.JSONBody(map[string]interface{}{"post_manually": true}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var a models.Assignment
	require.NoError(t, f.g.First(&a, gpA1).Error)
	assert.Equal(t, "manual", a.PostPolicy)
}

// G022: manual statuses are validated, tenant-scoped, and the missing
// policy scores past-due unsubmitted work.
func TestGradingPolicy_LateStatusAndMissingPolicy(t *testing.T) {
	f := newGPFixture(t)
	teacher := f.app(gpTeacher, 1, "TeacherEnrollment")
	url := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/late_policy_status", gpCourse, gpA2, gpStudentB)
	resp := testutil.MakeRequest(teacher, http.MethodPut, url, testutil.JSONBody(map[string]interface{}{"late_policy_status": "sideways"}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, url, testutil.JSONBody(map[string]interface{}{"late_policy_status": "late", "seconds_late_override": 3600}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, true, body["late"])
	assert.Equal(t, "late", body["late_policy_status"])

	// An outsider (not a student) and another course's assignment are 404s.
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/late_policy_status", gpCourse, gpA2, gpOutsider),
		testutil.JSONBody(map[string]interface{}{"late_policy_status": "missing"}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/late_policy_status", gpCourse, gpOtherTask, gpStudentB),
		testutil.JSONBody(map[string]interface{}{"late_policy_status": "missing"}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Missing policy: 30% off for missing work.
	require.NoError(t, f.g.Create(&models.LatePolicy{CourseID: gpCourse, MissingSubmissionDeductionEnabled: true, MissingSubmissionDeduction: 30}).Error)
	resp = testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/courses/%d/late_policy/apply", gpCourse), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var subs []models.Submission
	require.NoError(t, f.g.Where("assignment_id = ?", gpPastDue).Find(&subs).Error)
	require.Len(t, subs, 2)
	for _, s := range subs {
		assert.True(t, s.Missing)
		require.NotNil(t, s.Score)
		assert.InDelta(t, 7.0, *s.Score, 0.001)
	}
}

// G023: letter grades are read through the course scheme; G039: account
// schemes are validated and a course may only select its own or its
// account's.
func TestGradingPolicy_GradingTypesAndSchemes(t *testing.T) {
	f := newGPFixture(t)
	sub, err := f.sub.Grade(context.Background(), gpLetter, gpStudentA, gpTeacher, 1, "B")
	require.NoError(t, err)
	require.NotNil(t, sub.Score)
	assert.Equal(t, "B", *sub.Grade)
	assert.Greater(t, *sub.Score, 15.0)
	_, err = f.sub.Grade(context.Background(), gpLetter, gpStudentA, gpTeacher, 1, "Z-")
	assert.Error(t, err)

	admin := f.app(gpTeacher, 1, "")
	resp := testutil.MakeRequest(admin, http.MethodPost, "/accounts/1/grading_standards", testutil.JSONBody(map[string]interface{}{
		"grading_standard": map[string]interface{}{"title": "Pass scale", "data": [][]interface{}{{"Pass", 60}, {"Pass", 0}}},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "duplicate names")
	resp = testutil.MakeRequest(admin, http.MethodPost, "/accounts/1/grading_standards", testutil.JSONBody(map[string]interface{}{
		"grading_standard": map[string]interface{}{"title": "Pass scale", "data": [][]interface{}{{"Fail", 0}, {"Pass", 60}}},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created, _ := testutil.ParseJSONMap(resp)
	id := uint(created["id"].(float64))

	teacher := f.app(gpTeacher, 1, "TeacherEnrollment")
	resp = testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/%d/grading_scheme", gpCourse), testutil.JSONBody(map[string]interface{}{"grading_standard_id": id}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/grades", gpCourse), nil)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "Pass", gpStudent(t, body, gpStudentA)["current_grade"])

	// Another tenant's course cannot select this account's scheme.
	foreign := f.app(gpTeacherB, 2, "TeacherEnrollment")
	resp = testutil.MakeRequest(foreign, http.MethodPut, fmt.Sprintf("/courses/%d/grading_scheme", gpCourseB), testutil.JSONBody(map[string]interface{}{"grading_standard_id": id}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// G028: every attempt is kept; students do not see grades on an
// unposted submission's history.
func TestGradingPolicy_SubmissionHistory(t *testing.T) {
	f := newGPFixture(t)
	ctx := context.Background()
	for _, body := range []string{"first draft", "second draft"} {
		require.NoError(t, f.sub.Create(ctx, &models.Submission{AssignmentID: gpA1, UserID: gpStudentB, SubmissionType: strp("online_text_entry"), Body: strp(body)}))
	}
	teacher := f.app(gpTeacher, 1, "TeacherEnrollment")
	resp := testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/history", gpCourse, gpA1, gpStudentB), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	versions, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(versions), 2)
	assert.Equal(t, "first draft", versions[len(versions)-2]["body"])
	assert.Equal(t, "second draft", versions[len(versions)-1]["body"])

	classmate := f.app(gpStudentA, 1, "StudentEnrollment")
	resp = testutil.MakeRequest(classmate, http.MethodGet, fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/history", gpCourse, gpA1, gpStudentB), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// G040: an extra-days accommodation moves the student's effective due
// date, so work handed in inside the extension is not late.
func TestGradingPolicy_AccommodationExtendsDueDate(t *testing.T) {
	f := newGPFixture(t)
	days := 3
	require.NoError(t, f.g.Create(&models.StudentAccommodation{UserID: gpStudentA, CourseID: ptrUint(gpCourse), AccommodationType: "extended_time",
		ExtraDays: &days, Status: "active", CreatedByID: gpTeacher, EffectiveFrom: time.Now().Add(-48 * time.Hour)}).Error)
	dates, err := f.svc.StudentAssignmentDates(context.Background(), mustCourse(t, f.g, gpCourse), gpStudentA)
	require.NoError(t, err)
	d := dates[gpPastDue]
	require.NotNil(t, d.DueAt)
	assert.Equal(t, 3, d.ExtraDays)
	assert.True(t, d.DueAt.After(time.Now()), "extended past now")

	f.sub.SetAccommodationService(service.NewAccommodationService(postgres.NewStudentAccommodationRepository(f.g), postgres.NewAccommodationApplicationRepository(f.g)))
	sub := &models.Submission{AssignmentID: gpPastDue, UserID: gpStudentA, SubmissionType: strp("online_text_entry"), Body: strp("report")}
	require.NoError(t, f.sub.Create(context.Background(), sub))
	assert.False(t, sub.Late)
}

func ptrUint(v uint) *uint { return &v }

func mustCourse(t *testing.T, g *gorm.DB, id uint) *models.Course {
	t.Helper()
	var c models.Course
	require.NoError(t, g.First(&c, id).Error)
	return &c
}

// hide_final_grades hides the course total, grades and overrides from
// students on GET /grades (staff still see them); with "display totals for
// all grading periods" off the course-level override is withheld too.
func TestGradingPolicy_StudentTotalsVisibility(t *testing.T) {
	f := newGPFixture(t)
	require.NoError(t, f.g.Create(&models.CourseScoreOverride{CourseID: gpCourse, UserID: gpStudentA, OverrideScore: 88, UpdatedByID: gpTeacher}).Error)
	url := fmt.Sprintf("/courses/%d/grades", gpCourse)
	studentView := func() map[string]interface{} {
		resp := testutil.MakeRequest(f.app(gpStudentA, 1, "StudentEnrollment"), http.MethodGet, url, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := testutil.ParseJSONMap(resp)
		require.NoError(t, err)
		return gpStudent(t, body, gpStudentA)
	}
	me := studentView()
	assert.NotNil(t, me["current_score"])
	assert.Equal(t, 88.0, me["override_score"])

	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", gpCourse).Update("hide_final_grades", true).Error)
	me = studentView()
	assert.Nil(t, me["current_score"], "hide_final_grades")
	assert.Nil(t, me["final_score"])
	assert.Nil(t, me["override_score"])
	assert.Empty(t, me["current_grade"])
	resp := testutil.MakeRequest(f.app(gpTeacher, 1, "TeacherEnrollment"), http.MethodGet, url, nil)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	staff := gpStudent(t, body, gpStudentA)
	assert.NotNil(t, staff["current_score"], "staff still see totals")
	assert.Equal(t, 88.0, staff["override_score"])

	// Grading periods without "display totals for all grading periods":
	// no course-level total or override for the student.
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", gpCourse).Update("hide_final_grades", false).Error)
	now := time.Now()
	gpg := &models.GradingPeriodGroup{AccountID: 1, Title: "Year", WorkflowState: "active"}
	require.NoError(t, f.g.Create(gpg).Error)
	require.NoError(t, f.g.Create(&models.GradingPeriod{GradingPeriodGroupID: gpg.ID, Title: "Term", StartDate: now.AddDate(0, -1, 0),
		EndDate: now.AddDate(0, 1, 0), WorkflowState: "active"}).Error)
	term := &models.EnrollmentTerm{AccountID: 1, Name: "2026", GradingPeriodGroupID: &gpg.ID, WorkflowState: "active"}
	require.NoError(t, f.g.Create(term).Error)
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", gpCourse).Update("enrollment_term_id", term.ID).Error)
	me = studentView()
	assert.Nil(t, me["current_score"])
	assert.Nil(t, me["override_score"], "course-level override withheld with all-periods totals off")
	periods, _ := me["grading_periods"].([]interface{})
	require.Len(t, periods, 1)
	assert.NotNil(t, periods[0].(map[string]interface{})["current_score"], "per-period totals stay")
}
