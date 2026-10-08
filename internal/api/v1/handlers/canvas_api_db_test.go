package handlers_test

// DB-backed tests for the Canvas REST API compatibility endpoints (G137,
// G138): course rosters, account course/user lists, the multi-student
// submissions feed, SIS id resolution and account reports. Skipped when
// neither PARITY_DB_URL nor DATABASE_URL is set. All data is made up.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const (
	capiTeacher   = uint(601)
	capiStudent   = uint(602)
	capiInvited   = uint(603)
	capiObserver  = uint(604)
	capiOtherAcct = uint(605)
	capiAdmin     = uint(606)
	capiCourseA   = uint(700)
	capiCourseB   = uint(701)
	capiCourseX   = uint(702) // account 2
	capiTerm      = uint(800)
	capiPublished = uint(900)
	capiDraft     = uint(901)
)

type capiFixture struct {
	g *gorm.DB
}

func capiStrPtr(s string) *string { return &s }

func newCapiFixture(t *testing.T) *capiFixture {
	t.Helper()
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: capiTeacher, AccountID: 1, Name: "Terry Teacher", SortableName: "Teacher, Terry", LoginID: "terry.capi@example.test", Email: "terry.capi@example.test", PasswordHash: "x", Role: "user", SISUserID: capiStrPtr("T-1")},
		{ID: capiStudent, AccountID: 1, Name: "Ann Alpha", SortableName: "Alpha, Ann", LoginID: "ann.capi@example.test", Email: "ann.capi@example.test", PasswordHash: "x", Role: "user", SISUserID: capiStrPtr("S-1")},
		{ID: capiInvited, AccountID: 1, Name: "Bob Beta", SortableName: "Beta, Bob", LoginID: "bob.capi@example.test", Email: "bob.capi@example.test", PasswordHash: "x", Role: "user"},
		{ID: capiObserver, AccountID: 1, Name: "Olive Observer", SortableName: "Observer, Olive", LoginID: "olive.capi@example.test", Email: "olive.capi@example.test", PasswordHash: "x", Role: "user"},
		{ID: capiOtherAcct, AccountID: 2, Name: "Xavier Other", SortableName: "Other, Xavier", LoginID: "xavier.capi@example.test", Email: "xavier.capi@example.test", PasswordHash: "x", Role: "user", SISUserID: capiStrPtr("S-X")},
		{ID: capiAdmin, AccountID: 1, Name: "Alex Admin", SortableName: "Admin, Alex", LoginID: "alex.capi@example.test", Email: "alex.capi@example.test", PasswordHash: "x", Role: "admin"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	require.NoError(t, g.Create(&models.EnrollmentTerm{ID: capiTerm, AccountID: 1, Name: "Fall Term", SISTermID: capiStrPtr("FALL")}).Error)
	term := capiTerm
	for _, c := range []*models.Course{
		{ID: capiCourseA, AccountID: 1, Name: "Biology", CourseCode: "BIO-101", SISCourseID: capiStrPtr("BIO-101"), WorkflowState: models.CourseAvailable, EnrollmentTermID: &term, SyllabusBody: "<p>Read chapter 1</p>"},
		{ID: capiCourseB, AccountID: 1, Name: "Draft Chemistry", CourseCode: "CHEM", WorkflowState: models.CourseUnpublished},
		{ID: capiCourseX, AccountID: 2, Name: "History", CourseCode: "HIST", SISCourseID: capiStrPtr("HIST-1"), WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	student := capiStudent
	for _, e := range []*models.Enrollment{
		{UserID: capiTeacher, CourseID: capiCourseA, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: capiStudent, CourseID: capiCourseA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: capiInvited, CourseID: capiCourseA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentInvited},
		{UserID: capiObserver, CourseID: capiCourseA, Type: "ObserverEnrollment", Role: "ObserverEnrollment", WorkflowState: models.EnrollmentActive, AssociatedUserID: &student},
		{UserID: capiOtherAcct, CourseID: capiCourseX, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		e.PseudonymPoolCode = "animals_v1"
		require.NoError(t, g.Create(e).Error)
	}
	pts := 10.0
	require.NoError(t, g.Create(&models.Assignment{ID: capiPublished, CourseID: capiCourseA, Name: "Essay", PointsPossible: &pts, WorkflowState: models.AssignmentPublished, Published: true}).Error)
	require.NoError(t, g.Create(&models.Assignment{ID: capiDraft, CourseID: capiCourseA, Name: "Draft Quiz", WorkflowState: models.AssignmentUnpublished}).Error)
	score := 9.0
	grade := "9"
	graded := time.Now().Add(-time.Hour)
	submitted := time.Now().Add(-2 * time.Hour)
	for _, s := range []*models.Submission{
		// Graded but not posted: a student must not see the score.
		{AssignmentID: capiPublished, UserID: capiStudent, Score: &score, Grade: &grade, GradedAt: &graded, SubmittedAt: &submitted, WorkflowState: models.SubmissionGraded},
		{AssignmentID: capiPublished, UserID: capiInvited, SubmittedAt: &submitted, WorkflowState: models.SubmissionSubmitted},
		{AssignmentID: capiDraft, UserID: capiStudent, WorkflowState: models.SubmissionUnsubmitted},
	} {
		require.NoError(t, g.Create(s).Error)
	}
	return &capiFixture{g: g}
}

// fakeObservers links observer → students for the submissions feed.
type fakeObservers map[uint][]uint

func (f fakeObservers) IsObserverOf(_ context.Context, observer, student uint) (bool, error) {
	for _, s := range f[observer] {
		if s == student {
			return true, nil
		}
	}
	return false, nil
}

func capiCaller(userID, accountID uint, enrollmentType string, admin bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		c.Locals("is_admin", admin)
		if enrollmentType != "" {
			c.Locals("enrollment_type", enrollmentType)
		}
		return c.Next()
	}
}

func (f *capiFixture) app(userID, accountID uint, enrollmentType string, admin bool) *fiber.App {
	svc := service.NewCanvasAPIService(postgres.NewCanvasAPIRepository(f.g))
	h := handlers.NewCanvasAPIHandler(svc, fakeObservers{capiObserver: {capiStudent}})
	app := testutil.SetupTestApp()
	app.Use(capiCaller(userID, accountID, enrollmentType, admin), middleware.PaginationParams())
	app.Get("/courses/:course_id/users", h.ListCourseUsers)
	app.Get("/courses/:course_id/students/submissions", h.ListStudentSubmissions)
	app.Get("/accounts/:account_id/courses", h.ListAccountCourses)
	app.Get("/accounts/:account_id/users", h.ListAccountUsers)
	return app
}

func capiGet(t *testing.T, app *fiber.App, path string) (int, []map[string]interface{}) {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodGet, path, nil)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	arr, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err, path)
	return resp.StatusCode, arr
}

func capiIDs(rows []map[string]interface{}, key string) []int {
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		out = append(out, int(r[key].(float64)))
	}
	sort.Ints(out)
	return out
}

func TestCanvasAPI_CourseUsers(t *testing.T) {
	f := newCapiFixture(t)
	path := fmt.Sprintf("/courses/%d/users", capiCourseA)

	// Teacher: active + invited by default, with login/SIS ids.
	teacher := f.app(capiTeacher, 1, "TeacherEnrollment", false)
	code, rows := capiGet(t, teacher, path+"?include[]=enrollments&include[]=email")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, []int{int(capiTeacher), int(capiStudent), int(capiInvited), int(capiObserver)}, capiIDs(rows, "id"))
	for _, r := range rows {
		if uint(r["id"].(float64)) == capiStudent {
			assert.Equal(t, "S-1", r["sis_user_id"])
			assert.Equal(t, "ann.capi@example.test", r["login_id"])
			assert.Equal(t, "ann.capi@example.test", r["email"])
			enr := r["enrollments"].([]interface{})
			require.Len(t, enr, 1)
			assert.Equal(t, "StudentEnrollment", enr[0].(map[string]interface{})["type"])
		}
	}

	// Canvas short enrollment types and search.
	_, rows = capiGet(t, teacher, path+"?enrollment_type[]=student")
	assert.Equal(t, []int{int(capiStudent), int(capiInvited)}, capiIDs(rows, "id"))
	_, rows = capiGet(t, teacher, path+"?search_term=alph")
	assert.Equal(t, []int{int(capiStudent)}, capiIDs(rows, "id"))
	_, rows = capiGet(t, teacher, path+"?enrollment_state[]=invited")
	assert.Equal(t, []int{int(capiInvited)}, capiIDs(rows, "id"))
	code, _ = capiGet(t, teacher, path+"?enrollment_type[]=wizard")
	assert.Equal(t, http.StatusBadRequest, code)

	// Student: active members only, no login ids or emails, even when asked.
	student := f.app(capiStudent, 1, "StudentEnrollment", false)
	_, rows = capiGet(t, student, path+"?include[]=email&enrollment_state[]=invited")
	assert.Equal(t, []int{int(capiTeacher), int(capiStudent), int(capiObserver)}, capiIDs(rows, "id"))
	for _, r := range rows {
		assert.NotContains(t, r, "login_id")
		assert.NotContains(t, r, "email")
		assert.NotContains(t, r, "sis_user_id")
	}
}

func TestCanvasAPI_AccountCoursesAndUsers(t *testing.T) {
	f := newCapiFixture(t)
	admin := f.app(capiAdmin, 1, "", true)

	_, rows := capiGet(t, admin, "/accounts/1/courses")
	assert.Equal(t, []int{int(capiCourseA), int(capiCourseB)}, capiIDs(rows, "id"), "another tenant's course never appears")
	for _, r := range rows {
		assert.NotContains(t, r, "syllabus_body", "syllabus_body only with include[]")
	}

	_, rows = capiGet(t, admin, "/accounts/1/courses?published=true&include[]=total_students&include[]=term&include[]=teachers&include[]=syllabus_body")
	require.Len(t, rows, 1)
	a := rows[0]
	assert.Equal(t, "BIO-101", a["sis_course_id"])
	assert.Equal(t, float64(1), a["total_students"])
	assert.Equal(t, "Fall Term", a["term"].(map[string]interface{})["name"])
	teachers := a["teachers"].([]interface{})
	require.Len(t, teachers, 1)
	assert.Equal(t, "Terry Teacher", teachers[0].(map[string]interface{})["display_name"])
	assert.Equal(t, "<p>Read chapter 1</p>", a["syllabus_body"])

	_, rows = capiGet(t, admin, "/accounts/1/courses?search_term=chem")
	assert.Equal(t, []int{int(capiCourseB)}, capiIDs(rows, "id"))
	_, rows = capiGet(t, admin, fmt.Sprintf("/accounts/1/courses?enrollment_term_id=%d", capiTerm))
	assert.Equal(t, []int{int(capiCourseA)}, capiIDs(rows, "id"))
	_, rows = capiGet(t, admin, "/accounts/1/courses?with_enrollments=false")
	assert.Equal(t, []int{int(capiCourseB)}, capiIDs(rows, "id"))
	_, rows = capiGet(t, admin, fmt.Sprintf("/accounts/1/courses?by_teachers[]=%d", capiTeacher))
	assert.Equal(t, []int{int(capiCourseA)}, capiIDs(rows, "id"))
	code, _ := capiGet(t, admin, "/accounts/1/courses?state[]=bogus")
	assert.Equal(t, http.StatusBadRequest, code)

	_, rows = capiGet(t, admin, "/accounts/1/users?include[]=email")
	ids := capiIDs(rows, "id")
	assert.NotContains(t, ids, int(capiOtherAcct))
	assert.Contains(t, ids, int(capiStudent))
	_, rows = capiGet(t, admin, "/accounts/1/users?search_term=S-1")
	assert.Equal(t, []int{int(capiStudent)}, capiIDs(rows, "id"))
}

func TestCanvasAPI_StudentSubmissions(t *testing.T) {
	f := newCapiFixture(t)
	path := fmt.Sprintf("/courses/%d/students/submissions", capiCourseA)

	// Student default: own submissions on published assignments, with the
	// unposted grade withheld.
	student := f.app(capiStudent, 1, "StudentEnrollment", false)
	code, rows := capiGet(t, student, path+"?include[]=assignment")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, rows, 1)
	assert.Equal(t, float64(capiPublished), rows[0]["assignment_id"])
	assert.Nil(t, rows[0]["score"])
	assert.Equal(t, "submitted", rows[0]["workflow_state"])
	assert.Equal(t, "Essay", rows[0]["assignment"].(map[string]interface{})["name"])

	code, _ = capiGet(t, student, path+"?student_ids[]=all")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = capiGet(t, student, fmt.Sprintf("%s?student_ids[]=%d", path, capiInvited))
	assert.Equal(t, http.StatusForbidden, code)

	// Observer: a linked student only.
	observer := f.app(capiObserver, 1, "ObserverEnrollment", false)
	code, rows = capiGet(t, observer, fmt.Sprintf("%s?student_ids[]=%d", path, capiStudent))
	require.Equal(t, http.StatusOK, code)
	assert.Len(t, rows, 1)
	code, _ = capiGet(t, observer, fmt.Sprintf("%s?student_ids[]=%d", path, capiInvited))
	assert.Equal(t, http.StatusForbidden, code)

	// Teacher: everyone, unpublished assignments included, real scores.
	teacher := f.app(capiTeacher, 1, "TeacherEnrollment", false)
	_, rows = capiGet(t, teacher, path+"?student_ids[]=all")
	assert.Len(t, rows, 3)
	_, rows = capiGet(t, teacher, path+"?student_ids[]=all&workflow_state=graded")
	require.Len(t, rows, 1)
	assert.Equal(t, 9.0, rows[0]["score"])
	_, rows = capiGet(t, teacher, fmt.Sprintf("%s?student_ids[]=all&assignment_ids[]=%d&grouped=true", path, capiPublished))
	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.Len(t, r["submissions"].([]interface{}), 1)
	}
}

func TestCanvasAPI_ResolveSISIDs(t *testing.T) {
	f := newCapiFixture(t)
	svc := service.NewCanvasAPIService(postgres.NewCanvasAPIRepository(f.g))
	ctx := context.Background()

	id, err := svc.ResolveCanvasID(ctx, "sis_course_id", "BIO-101", 1)
	require.NoError(t, err)
	assert.Equal(t, capiCourseA, id)
	_, err = svc.ResolveCanvasID(ctx, "sis_course_id", "HIST-1", 1)
	assert.Error(t, err, "another tenant's SIS id must not resolve")
	id, err = svc.ResolveCanvasID(ctx, "sis_course_id", "HIST-1", 0)
	require.NoError(t, err, "unscoped (super admin) lookup")
	assert.Equal(t, capiCourseX, id)

	id, err = svc.ResolveCanvasID(ctx, "sis_user_id", "S-1", 1)
	require.NoError(t, err)
	assert.Equal(t, capiStudent, id)
	id, err = svc.ResolveCanvasID(ctx, "sis_login_id", "bob.capi@example.test", 1)
	require.NoError(t, err)
	assert.Equal(t, capiInvited, id)
	id, err = svc.ResolveCanvasID(ctx, "sis_term_id", "FALL", 1)
	require.NoError(t, err)
	assert.Equal(t, capiTerm, id)

	// End to end through the middleware: /courses/sis_course_id:BIO-101/users.
	h := handlers.NewCanvasAPIHandler(svc, nil)
	app := testutil.SetupTestApp()
	api := app.Group("/api/v1", middleware.PaginationParams())
	protected := api.Group("", capiCaller(capiTeacher, 1, "TeacherEnrollment", false))
	protected.Use(middleware.CanvasIDs(svc, postgres.NewUserRepository(f.g)))
	protected.Get("/courses/:course_id/users", h.ListCourseUsers)
	code, rows := capiGet(t, app, "/api/v1/courses/sis_course_id:BIO-101/users?enrollment_type[]=teacher")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, []int{int(capiTeacher)}, capiIDs(rows, "id"))
	code, _ = capiGet(t, app, "/api/v1/courses/sis_course_id:HIST-1/users")
	assert.Equal(t, http.StatusNotFound, code)
}

// G137: GET /courses and /courses/:id carry Canvas's "enrollments" array
// (short types), sis_course_id for staff only, and include[]=term/teachers.
func TestCanvasAPI_CourseIncludes(t *testing.T) {
	f := newCapiFixture(t)
	courseRepo := postgres.NewCourseRepository(f.g)
	enrollmentRepo := postgres.NewEnrollmentRepository(f.g)
	h := handlers.NewCourseHandler(service.NewCourseService(courseRepo, enrollmentRepo, postgres.NewSectionRepository(f.g)), service.NewEnrollmentService(enrollmentRepo)).
		SetUserRepo(postgres.NewUserRepository(f.g)).
		SetCanvasAPI(service.NewCanvasAPIService(postgres.NewCanvasAPIRepository(f.g)))
	mk := func(userID uint) *fiber.App {
		app := testutil.SetupTestApp()
		app.Use(capiCaller(userID, 1, "", false), middleware.PaginationParams())
		app.Get("/courses", h.ListCourses)
		app.Get("/courses/:id", h.GetCourse)
		return app
	}

	resp := testutil.MakeRequest(mk(capiTeacher), http.MethodGet, fmt.Sprintf("/courses/%d?include[]=term&include[]=teachers", capiCourseA), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "BIO-101", body["sis_course_id"])
	enr := body["enrollments"].([]interface{})
	require.Len(t, enr, 1)
	assert.Equal(t, "teacher", enr[0].(map[string]interface{})["type"])
	assert.Equal(t, "Fall Term", body["term"].(map[string]interface{})["name"])
	assert.Len(t, body["teachers"].([]interface{}), 1)

	_, rows := capiGet(t, mk(capiStudent), "/courses")
	require.Len(t, rows, 1)
	assert.NotContains(t, rows[0], "sis_course_id", "students don't see SIS ids")
	assert.Equal(t, "student", rows[0]["enrollments"].([]interface{})[0].(map[string]interface{})["type"])
	assert.NotContains(t, rows[0], "term", "term only with include[]")
}

// fakeSIS stands in for the SIS CSV exporter.
type fakeSIS struct{}

func (fakeSIS) ExportUsersCSV(context.Context, uint) ([]byte, error) {
	return []byte("user_id,login_id\nS-1,ann\n"), nil
}
func (fakeSIS) ExportCoursesCSV(context.Context, uint) ([]byte, error) {
	return []byte("course_id\nBIO-101\n"), nil
}
func (fakeSIS) ExportSectionsCSV(context.Context, uint) ([]byte, error) {
	return []byte("section_id\n"), nil
}
func (fakeSIS) ExportEnrollmentsCSV(context.Context, uint) ([]byte, error) {
	return []byte("course_id,user_id\n"), nil
}

type fixedGrader struct{}

func (fixedGrader) GetStudentGrade(context.Context, uint, uint) (*service.StudentGrade, error) {
	return &service.StudentGrade{CurrentScore: 90, FinalScore: 45}, nil
}

func TestAccountReports_RunAndDownload(t *testing.T) {
	f := newCapiFixture(t)
	backend := storage.NewLocalBackend(t.TempDir())
	svc := service.NewAccountReportService(postgres.NewAccountReportRepository(f.g), backend, fakeSIS{}, fixedGrader{})
	svc.SetRunner(func(job func()) { job() }) // run inline
	h := handlers.NewAccountReportHandler(svc)

	mk := func(userID, accountID uint) *fiber.App {
		app := testutil.SetupTestApp()
		app.Use(capiCaller(userID, accountID, "", true), middleware.PaginationParams())
		app.Get("/accounts/:account_id/reports", h.ListAvailable)
		app.Get("/accounts/:account_id/reports/:report", h.ListRuns)
		app.Post("/accounts/:account_id/reports/:report", h.Start)
		app.Get("/accounts/:account_id/reports/:report/:id", h.GetRun)
		app.Get("/accounts/:account_id/reports/:report/:id/file", h.DownloadRun)
		app.Delete("/accounts/:account_id/reports/:report/:id", h.DeleteRun)
		return app
	}
	app := mk(capiAdmin, 1)

	_, rows := capiGet(t, app, "/accounts/1/reports")
	var names []string
	for _, r := range rows {
		names = append(names, r["report"].(string))
	}
	assert.Contains(t, names, "provisioning_csv")
	assert.Contains(t, names, "grade_export_csv")

	// Grade export: one row per student enrollment of this account.
	resp := testutil.MakeRequest(app, http.MethodPost, "/accounts/1/reports/grade_export_csv", testutil.JSONBody(map[string]interface{}{"parameters": map[string]interface{}{}}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	run, _ := testutil.ParseJSONMap(resp)
	runID := int(run["id"].(float64))
	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/accounts/1/reports/grade_export_csv/%d", runID), nil)
	status, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "complete", status["status"])
	assert.NotNil(t, status["file_url"])

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, fmt.Sprintf("/accounts/1/reports/grade_export_csv/%d/file", runID), nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	assert.Contains(t, text, "Ann Alpha")
	assert.Contains(t, text, "90.00")
	assert.NotContains(t, text, "Xavier Other", "another tenant's students never appear")

	// Provisioning with two parts is a ZIP; with none it is refused.
	resp = testutil.MakeRequest(app, http.MethodPost, "/accounts/1/reports/provisioning_csv", testutil.JSONBody(map[string]interface{}{
		"parameters": map[string]interface{}{"users": true, "courses": "true"},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	run, _ = testutil.ParseJSONMap(resp)
	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/accounts/1/reports/provisioning_csv/%d", int(run["id"].(float64))), nil)
	status, _ = testutil.ParseJSONMap(resp)
	assert.Equal(t, "application/zip", status["attachment"].(map[string]interface{})["content-type"])
	resp = testutil.MakeRequest(app, http.MethodPost, "/accounts/1/reports/provisioning_csv", testutil.JSONBody(map[string]interface{}{"parameters": map[string]interface{}{}}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, "/accounts/1/reports/not_a_report", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Another tenant's admin cannot see or fetch the run.
	other := mk(capiOtherAcct, 2)
	resp = testutil.MakeRequest(other, http.MethodGet, fmt.Sprintf("/accounts/2/reports/grade_export_csv/%d", runID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(other, http.MethodGet, fmt.Sprintf("/accounts/2/reports/grade_export_csv/%d/file", runID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// The run must be named under its own report type.
	resp = testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/accounts/1/reports/provisioning_csv/%d", runID), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Delete removes it from the history.
	resp = testutil.MakeRequest(app, http.MethodDelete, fmt.Sprintf("/accounts/1/reports/grade_export_csv/%d", runID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_, rows = capiGet(t, app, "/accounts/1/reports/grade_export_csv")
	assert.Len(t, rows, 0)

	// Form-encoded Canvas parameters[...] also work.
	req := httptest.NewRequest(http.MethodPost, "/accounts/1/reports/provisioning_csv", strings.NewReader("parameters[users]=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
