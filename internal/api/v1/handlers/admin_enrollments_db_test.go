package handlers_test

// DB-backed tests for the admin-enrollments lane: enrollment lifecycle
// (G105), date-based access (G106), section-limited enrollments (G107),
// cross-listing (G108), section management (G109), multiple enrollments
// per course (G110) and course invitations (G122). The app mounts the
// real permission middleware, handlers, services and Postgres
// repositories. All data is made up.

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
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const (
	aeTeacher   = uint(40)
	aeTA        = uint(41)
	aeStudentA  = uint(42)
	aeStudentB  = uint(43)
	aeOutsider  = uint(44)
	aeAdmin     = uint(45)
	aeTeacherT2 = uint(46)
	aeCourse    = uint(400)
	aeCourse2   = uint(401)
	aeCourseT2  = uint(500)
	aeSectionA  = uint(4000)
	aeSectionB  = uint(4001)
	aeSectionX  = uint(4010) // belongs to aeCourse2
)

type aeFixture struct {
	g           *gorm.DB
	enrollments map[string]uint // label -> enrollment id
	// notifier, when set, receives course invitation notifications.
	notifier service.Notifier
}

func newAEFixture(t *testing.T) *aeFixture {
	t.Helper()
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: aeTeacher, AccountID: 1, Name: "Terry Teacher", LoginID: "terry.ae@example.test", Email: "terry.ae@example.test", PasswordHash: "x", Role: "user"},
		{ID: aeTA, AccountID: 1, Name: "Tia Assistant", LoginID: "tia.ae@example.test", Email: "tia.ae@example.test", PasswordHash: "x", Role: "user"},
		{ID: aeStudentA, AccountID: 1, Name: "Sam Student", LoginID: "sam.ae@example.test", Email: "sam.ae@example.test", PasswordHash: "x", Role: "user"},
		{ID: aeStudentB, AccountID: 1, Name: "Bea Student", LoginID: "bea.ae@example.test", Email: "bea.ae@example.test", PasswordHash: "x", Role: "user"},
		{ID: aeOutsider, AccountID: 1, Name: "Olly Outsider", LoginID: "olly.ae@example.test", Email: "olly.ae@example.test", PasswordHash: "x", Role: "user"},
		{ID: aeAdmin, AccountID: 1, Name: "Ada Admin", LoginID: "ada.ae@example.test", Email: "ada.ae@example.test", PasswordHash: "x", Role: "admin"},
		{ID: aeTeacherT2, AccountID: 2, Name: "Tom Teacher", LoginID: "tom.ae@example.test", Email: "tom.ae@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	for _, c := range []*models.Course{
		{ID: aeCourse, AccountID: 1, Name: "Biology", CourseCode: "BIO", WorkflowState: models.CourseAvailable},
		{ID: aeCourse2, AccountID: 1, Name: "Biology Lab", CourseCode: "BIO-L", WorkflowState: models.CourseAvailable},
		{ID: aeCourseT2, AccountID: 2, Name: "History", CourseCode: "HIST", WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	for _, s := range []*models.CourseSection{
		{ID: aeSectionA, CourseID: aeCourse, Name: "Section A", WorkflowState: "active"},
		{ID: aeSectionB, CourseID: aeCourse, Name: "Section B", WorkflowState: "active"},
		{ID: aeSectionX, CourseID: aeCourse2, Name: "Lab", WorkflowState: "active"},
	} {
		require.NoError(t, g.Create(s).Error)
	}
	f := &aeFixture{g: g, enrollments: map[string]uint{}}
	sa, sb := aeSectionA, aeSectionB
	for label, e := range map[string]*models.Enrollment{
		"teacher":   {UserID: aeTeacher, CourseID: aeCourse, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		"teacher2":  {UserID: aeTeacher, CourseID: aeCourse2, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		"ta":        {UserID: aeTA, CourseID: aeCourse, CourseSectionID: &sa, Type: "TaEnrollment", Role: "TaEnrollment", WorkflowState: models.EnrollmentActive, LimitPrivilegesToCourseSection: true},
		"studentA":  {UserID: aeStudentA, CourseID: aeCourse, CourseSectionID: &sa, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		"studentB":  {UserID: aeStudentB, CourseID: aeCourse, CourseSectionID: &sb, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		"teacherT2": {UserID: aeTeacherT2, CourseID: aeCourseT2, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
		f.enrollments[label] = e.ID
	}
	return f
}

func ok200(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) }

// app mounts the lane's routes (mirroring routes_admin_enrollments.go and
// the existing roster routes) plus probe routes standing in for ordinary
// course content, as the given caller.
func (f *aeFixture) app(userID, accountID uint) *fiber.App {
	g := f.g
	users := postgres.NewUserRepository(g)
	courses := postgres.NewCourseRepository(g)
	enrollRepo := postgres.NewEnrollmentRepository(g)
	sectionRepo := postgres.NewSectionRepository(g)
	adminRepo := postgres.NewEnrollmentAdminRepository(g)

	pm := middleware.NewPermissionMiddleware(enrollRepo, users, courses)
	authz := handlers.NewResourceAuthorizer(enrollRepo, users, courses)
	lifecycle := service.NewEnrollmentLifecycleService(adminRepo, sectionRepo)
	if f.notifier != nil {
		lifecycle.SetNotifier(f.notifier)
	}
	enrollSvc := service.NewEnrollmentService(enrollRepo).SetTenantRepos(users, courses, sectionRepo).SetLifecycle(lifecycle)
	eh := handlers.NewEnrollmentHandler(enrollSvc).SetLifecycle(lifecycle)
	sh := handlers.NewSectionHandler(sectionRepo, authz).
		SetSectionService(service.NewSectionService(postgres.NewSectionAdminRepository(g), adminRepo, courses))

	enrolled := pm.RequireEnrolled()
	instructor := pm.RequireInstructor()

	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	api := app.Group("/api/v1")
	api.Get("/courses/:course_id/enrollments", enrolled, eh.ListEnrollments)
	api.Post("/courses/:course_id/enrollments", instructor, eh.CreateEnrollment)
	api.Put("/courses/:course_id/enrollments/:id", instructor, eh.UpdateEnrollment)
	api.Delete("/courses/:course_id/enrollments/:id", instructor, eh.DeleteEnrollment)
	api.Put("/courses/:course_id/enrollments/:id/reactivate", instructor, eh.ReactivateEnrollment)
	api.Post("/courses/:course_id/enrollments/:id/accept", eh.AcceptInvitation)
	api.Post("/courses/:course_id/enrollments/:id/reject", eh.RejectInvitation)
	api.Get("/users/self/course_invitations", eh.ListMyInvitations)
	api.Put("/sections/:id", sh.UpdateSection)
	api.Delete("/sections/:id", sh.DeleteSection)
	api.Post("/sections/:id/crosslist/:new_course_id", sh.CrosslistSection)
	api.Delete("/sections/:id/crosslist", sh.UncrosslistSection)
	// Probes.
	api.Get("/courses/:course_id/modules", enrolled, ok200)
	api.Post("/courses/:course_id/modules", enrolled, ok200)
	api.Get("/courses/:course_id/students/:student_id/grade", instructor, ok200)
	api.Get("/courses/:course_id/teach", instructor, ok200)
	api.Get("/authz/courses/:course_id", func(c *fiber.Ctx) error {
		id, _ := c.ParamsInt("course_id")
		if err := authz.RequireCourseEnrolled(c, uint(id)); err != nil {
			return err
		}
		return ok200(c)
	})
	api.Post("/authz/courses/:course_id", func(c *fiber.Ctx) error {
		id, _ := c.ParamsInt("course_id")
		if err := authz.RequireCourseEnrolled(c, uint(id)); err != nil {
			return err
		}
		return ok200(c)
	})
	return app
}

func aeDo(t *testing.T, app *fiber.App, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var resp *http.Response
	if body != nil {
		resp = testutil.MakeRequest(app, method, path, testutil.JSONBody(body))
	} else {
		resp = testutil.MakeRequest(app, method, path, nil)
	}
	require.NotNil(t, resp)
	out := map[string]interface{}{}
	_ = testutil.ParseJSONResponse(resp, &out)
	return resp.StatusCode, out
}

func aeList(t *testing.T, app *fiber.App, path string) (int, []map[string]interface{}) {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodGet, path, nil)
	require.NotNil(t, resp)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	arr, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	return resp.StatusCode, arr
}

func (f *aeFixture) state(t *testing.T, label string) string {
	t.Helper()
	var e models.Enrollment
	require.NoError(t, f.g.First(&e, f.enrollments[label]).Error)
	return string(e.WorkflowState)
}

func enrollmentPath(course, id uint) string {
	return fmt.Sprintf("/api/v1/courses/%d/enrollments/%d", course, id)
}

// ---------------------------------------------------------------------------
// G105 — lifecycle
// ---------------------------------------------------------------------------

func TestAdminEnrollments_LifecycleConcludeDeactivateReactivateDelete(t *testing.T) {
	f := newAEFixture(t)
	teacher := f.app(aeTeacher, 1)
	student := f.app(aeStudentA, 1)
	path := enrollmentPath(aeCourse, f.enrollments["studentA"])
	modules := fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse)

	// Conclude (the default task): read-only access remains.
	code, body := aeDo(t, teacher, http.MethodDelete, path, nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "completed", body["enrollment_state"])
	assert.NotNil(t, body["completed_at"])
	code, _ = aeDo(t, student, http.MethodGet, modules, nil)
	assert.Equal(t, http.StatusOK, code, "a concluded student can still view")
	code, _ = aeDo(t, student, http.MethodPost, modules, nil)
	assert.Equal(t, http.StatusForbidden, code, "a concluded student can't participate")
	code, _ = aeDo(t, student, http.MethodPost, fmt.Sprintf("/api/v1/authz/courses/%d", aeCourse), nil)
	assert.Equal(t, http.StatusForbidden, code, "handler-level authz is read-only too")

	// Deactivate: no access at all.
	code, _ = aeDo(t, teacher, http.MethodDelete, path+"?task=deactivate", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "inactive", f.state(t, "studentA"))
	code, _ = aeDo(t, student, http.MethodGet, modules, nil)
	assert.Equal(t, http.StatusForbidden, code)

	// Reactivate: full access again.
	code, _ = aeDo(t, teacher, http.MethodPut, path+"/reactivate", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "active", f.state(t, "studentA"))
	code, _ = aeDo(t, student, http.MethodPost, modules, nil)
	assert.Equal(t, http.StatusOK, code)

	// Reactivating an active enrollment is refused.
	code, _ = aeDo(t, teacher, http.MethodPut, path+"/reactivate", nil)
	assert.Equal(t, http.StatusBadRequest, code)
	// Unknown task.
	code, _ = aeDo(t, teacher, http.MethodDelete, path+"?task=explode", nil)
	assert.Equal(t, http.StatusBadRequest, code)

	// Delete: gone from the roster and from the course.
	code, _ = aeDo(t, teacher, http.MethodDelete, path+"?task=delete", nil)
	require.Equal(t, http.StatusOK, code)
	_, roster := aeList(t, teacher, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse))
	for _, e := range roster {
		assert.NotEqual(t, float64(aeStudentA), e["user_id"], "deleted enrollment must leave the roster")
	}
	code, _ = aeDo(t, student, http.MethodGet, modules, nil)
	assert.Equal(t, http.StatusForbidden, code)
	// A deleted enrollment can't be managed any more.
	code, _ = aeDo(t, teacher, http.MethodPut, path+"/reactivate", nil)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestAdminEnrollments_LifecyclePermissions(t *testing.T) {
	f := newAEFixture(t)
	// A student can't manage enrollments (instructor gate).
	code, _ := aeDo(t, f.app(aeStudentA, 1), http.MethodDelete, enrollmentPath(aeCourse, f.enrollments["studentB"]), nil)
	assert.Equal(t, http.StatusForbidden, code)
	// A TA can't conclude the teacher.
	code, _ = aeDo(t, f.app(aeTA, 1), http.MethodDelete, enrollmentPath(aeCourse, f.enrollments["teacher"]), nil)
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "active", f.state(t, "teacher"))
	// Nobody but an admin removes themselves.
	code, _ = aeDo(t, f.app(aeTeacher, 1), http.MethodDelete, enrollmentPath(aeCourse, f.enrollments["teacher"]), nil)
	assert.Equal(t, http.StatusForbidden, code)
	// Another tenant's teacher: 404 (the course is not in their tenant).
	code, _ = aeDo(t, f.app(aeTeacherT2, 2), http.MethodDelete, enrollmentPath(aeCourse, f.enrollments["studentA"]), nil)
	assert.Equal(t, http.StatusNotFound, code)
	// Course parent-tie: an enrollment id of another course is a 404.
	code, _ = aeDo(t, f.app(aeTeacher, 1), http.MethodDelete, enrollmentPath(aeCourse2, f.enrollments["studentA"]), nil)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "active", f.state(t, "studentA"))
	// The tenant admin can manage anyone.
	code, _ = aeDo(t, f.app(aeAdmin, 1), http.MethodDelete, enrollmentPath(aeCourse, f.enrollments["teacher"])+"?task=deactivate", nil)
	assert.Equal(t, http.StatusOK, code)
}

func TestAdminEnrollments_EditRoleSectionAndDates(t *testing.T) {
	f := newAEFixture(t)
	teacher := f.app(aeTeacher, 1)
	path := enrollmentPath(aeCourse, f.enrollments["studentB"])

	code, body := aeDo(t, teacher, http.MethodPut, path, map[string]interface{}{
		"enrollment": map[string]interface{}{"type": "TaEnrollment", "course_section_id": aeSectionA},
	})
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "TaEnrollment", body["type"])
	assert.Equal(t, float64(aeSectionA), body["course_section_id"])

	// Clear the section (default section).
	code, body = aeDo(t, teacher, http.MethodPut, path, map[string]interface{}{
		"enrollment": map[string]interface{}{"course_section_id": nil},
	})
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, body["course_section_id"])

	// A section of another course is refused.
	code, _ = aeDo(t, teacher, http.MethodPut, path, map[string]interface{}{
		"enrollment": map[string]interface{}{"course_section_id": aeSectionX},
	})
	assert.Equal(t, http.StatusNotFound, code)

	// Dates: end before start is a 400.
	code, _ = aeDo(t, teacher, http.MethodPut, path, map[string]interface{}{
		"enrollment": map[string]interface{}{"start_at": "2026-09-01T00:00:00Z", "end_at": "2026-08-01T00:00:00Z"},
	})
	assert.Equal(t, http.StatusBadRequest, code)

	// A TA can't promote a student to teacher.
	code, _ = aeDo(t, f.app(aeAdmin, 1), http.MethodPut, path, map[string]interface{}{
		"enrollment": map[string]interface{}{"type": "StudentEnrollment"},
	})
	require.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, f.app(aeTA, 1), http.MethodPut, enrollmentPath(aeCourse, f.enrollments["studentA"]), map[string]interface{}{
		"enrollment": map[string]interface{}{"type": "TeacherEnrollment"},
	})
	assert.Equal(t, http.StatusForbidden, code)
}

// ---------------------------------------------------------------------------
// G106 — date-based access
// ---------------------------------------------------------------------------

func TestAdminEnrollments_DateBasedAccess(t *testing.T) {
	f := newAEFixture(t)
	student := f.app(aeStudentB, 1)
	modules := fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse)
	past, longAgo, future, later := time.Now().Add(-24*time.Hour), time.Now().Add(-30*24*time.Hour), time.Now().Add(24*time.Hour), time.Now().Add(30*24*time.Hour)

	// Enrollment dates in the past: concluded -> read-only.
	require.NoError(t, f.g.Model(&models.Enrollment{}).Where("id = ?", f.enrollments["studentB"]).
		Updates(map[string]interface{}{"start_at": longAgo, "end_at": past}).Error)
	code, _ := aeDo(t, student, http.MethodGet, modules, nil)
	assert.Equal(t, http.StatusOK, code)
	code, body := aeDo(t, student, http.MethodPost, modules, nil)
	assert.Equal(t, http.StatusForbidden, code, body)

	// The course hides concluded courses from students.
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", aeCourse).Update("restrict_student_past_view", true).Error)
	code, _ = aeDo(t, student, http.MethodGet, modules, nil)
	assert.Equal(t, http.StatusForbidden, code)

	// Future enrollment dates: pending -> may view (future view allowed).
	require.NoError(t, f.g.Model(&models.Enrollment{}).Where("id = ?", f.enrollments["studentB"]).
		Updates(map[string]interface{}{"start_at": future, "end_at": later}).Error)
	code, _ = aeDo(t, student, http.MethodGet, modules, nil)
	assert.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, student, http.MethodPost, modules, nil)
	assert.Equal(t, http.StatusForbidden, code)

	// Clear enrollment dates; a term that has ended concludes the course.
	require.NoError(t, f.g.Model(&models.Enrollment{}).Where("id = ?", f.enrollments["studentB"]).
		Updates(map[string]interface{}{"start_at": nil, "end_at": nil}).Error)
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", aeCourse).Update("restrict_student_past_view", false).Error)
	term := &models.EnrollmentTerm{AccountID: 1, Name: "Spring", StartAt: &longAgo, EndAt: &past, WorkflowState: "active"}
	require.NoError(t, f.g.Create(term).Error)
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", aeCourse).Update("enrollment_term_id", term.ID).Error)
	code, _ = aeDo(t, student, http.MethodPost, modules, nil)
	assert.Equal(t, http.StatusForbidden, code, "term ended: read-only")
	// Restricted section dates that are current override the term.
	require.NoError(t, f.g.Model(&models.CourseSection{}).Where("id = ?", aeSectionB).
		Updates(map[string]interface{}{"start_at": longAgo, "end_at": later, "restrict_enrollments_to_section_dates": true}).Error)
	code, _ = aeDo(t, student, http.MethodPost, modules, nil)
	assert.Equal(t, http.StatusOK, code, "section dates override the term")

	// Restricted course dates in the future: the teacher keeps the
	// (dateless default) term window and can still prepare the course,
	// and can still undo the setting.
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", aeCourse2).
		Updates(map[string]interface{}{"start_at": future, "end_at": later, "restrict_enrollments_to_course_dates": true}).Error)
	code, _ = aeDo(t, f.app(aeTeacher, 1), http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse2), nil)
	assert.Equal(t, http.StatusOK, code, "staff are not locked out by future course dates")
	// Staff whose own enrollment dates are in the future have no access.
	require.NoError(t, f.g.Model(&models.Enrollment{}).Where("id = ?", f.enrollments["teacher2"]).
		Updates(map[string]interface{}{"start_at": future, "end_at": later}).Error)
	code, _ = aeDo(t, f.app(aeTeacher, 1), http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse2), nil)
	assert.Equal(t, http.StatusForbidden, code, "staff before their own enrollment dates have no access")
	// Admins are never date-restricted.
	code, _ = aeDo(t, f.app(aeAdmin, 1), http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse2), nil)
	assert.Equal(t, http.StatusOK, code)
}

func TestAdminEnrollments_ConcludedCourseIsReadOnly(t *testing.T) {
	f := newAEFixture(t)
	require.NoError(t, f.g.Model(&models.Course{}).Where("id = ?", aeCourse).Update("workflow_state", models.CourseCompleted).Error)
	modules := fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse)
	for _, uid := range []uint{aeStudentA, aeTeacher} {
		app := f.app(uid, 1)
		code, _ := aeDo(t, app, http.MethodGet, modules, nil)
		assert.Equal(t, http.StatusOK, code)
		code, _ = aeDo(t, app, http.MethodPost, modules, nil)
		assert.Equal(t, http.StatusForbidden, code)
	}
}

// ---------------------------------------------------------------------------
// G107 — section-limited enrollments
// ---------------------------------------------------------------------------

func TestAdminEnrollments_SectionLimitedTA(t *testing.T) {
	f := newAEFixture(t)
	ta := f.app(aeTA, 1)

	code, roster := aeList(t, ta, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse))
	require.Equal(t, http.StatusOK, code)
	seen := map[float64]bool{}
	for _, e := range roster {
		seen[e["user_id"].(float64)] = true
	}
	assert.True(t, seen[float64(aeStudentA)], "own section's student visible")
	assert.False(t, seen[float64(aeStudentB)], "other section's student hidden")
	assert.True(t, seen[float64(aeTeacher)], "course staff stay visible")

	code, _ = aeDo(t, ta, http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/students/%d/grade", aeCourse, aeStudentA), nil)
	assert.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, ta, http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/students/%d/grade", aeCourse, aeStudentB), nil)
	assert.Equal(t, http.StatusNotFound, code, "per-user routes outside the TA's section are hidden")
	// Nor can the TA manage the other section's enrollment.
	code, _ = aeDo(t, ta, http.MethodDelete, enrollmentPath(aeCourse, f.enrollments["studentB"]), nil)
	assert.Equal(t, http.StatusNotFound, code)
	// ...or move their own student into it.
	code, _ = aeDo(t, ta, http.MethodPut, enrollmentPath(aeCourse, f.enrollments["studentA"]), map[string]interface{}{
		"enrollment": map[string]interface{}{"course_section_id": aeSectionB},
	})
	assert.Equal(t, http.StatusNotFound, code)
	// ...or add someone to another section, though their own works.
	addAs := func(app *fiber.App, body map[string]interface{}) int {
		code, _ := aeDo(t, app, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse), map[string]interface{}{"enrollment": body})
		return code
	}
	assert.Equal(t, http.StatusNotFound, addAs(ta, map[string]interface{}{"user_id": aeOutsider, "type": "StudentEnrollment", "course_section_id": aeSectionB}))
	assert.Equal(t, http.StatusNotFound, addAs(ta, map[string]interface{}{"user_id": aeOutsider, "type": "StudentEnrollment"}), "default section is not theirs either")
	// Nor widen their own access with an unlimited student role (G110).
	assert.Equal(t, http.StatusForbidden, addAs(ta, map[string]interface{}{"user_id": aeTA, "type": "StudentEnrollment", "course_section_id": aeSectionA}))
	assert.Equal(t, http.StatusCreated, addAs(ta, map[string]interface{}{"user_id": aeOutsider, "type": "StudentEnrollment", "course_section_id": aeSectionA}))

	require.NoError(t, f.g.Where("user_id = ? AND course_id = ? AND type = ?", aeOutsider, aeCourse, "StudentEnrollment").Delete(&models.Enrollment{}).Error)

	// A section-limited teacher sees other sections' staff on the roster
	// but can't manage their enrollments.
	sa, sb := aeSectionA, aeSectionB
	limitedTeacher := &models.Enrollment{UserID: aeOutsider, CourseID: aeCourse, CourseSectionID: &sa, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive, LimitPrivilegesToCourseSection: true}
	otherTA := &models.Enrollment{UserID: aeStudentA, CourseID: aeCourse, CourseSectionID: &sb, Type: "TaEnrollment", Role: "TaEnrollment", WorkflowState: models.EnrollmentActive}
	require.NoError(t, f.g.Create(limitedTeacher).Error)
	require.NoError(t, f.g.Create(otherTA).Error)
	code, _ = aeDo(t, f.app(aeOutsider, 1), http.MethodDelete, enrollmentPath(aeCourse, otherTA.ID)+"?task=deactivate", nil)
	assert.Equal(t, http.StatusNotFound, code)
	var stillActive models.Enrollment
	require.NoError(t, f.g.First(&stillActive, otherTA.ID).Error)
	assert.Equal(t, models.EnrollmentActive, stillActive.WorkflowState)
	require.NoError(t, f.g.Delete(limitedTeacher).Error)
	require.NoError(t, f.g.Delete(otherTA).Error)

	// The unlimited teacher sees everyone.
	_, roster = aeList(t, f.app(aeTeacher, 1), fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse))
	assert.Len(t, roster, 4)
}

func TestAdminEnrollments_StudentRosterStates(t *testing.T) {
	f := newAEFixture(t)
	require.NoError(t, f.g.Model(&models.Enrollment{}).Where("id = ?", f.enrollments["studentB"]).Update("workflow_state", "inactive").Error)
	_, roster := aeList(t, f.app(aeStudentA, 1), fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse))
	for _, e := range roster {
		assert.NotEqual(t, "inactive", e["enrollment_state"], "students only see active and invited people")
		_, hasEmail := e["user"].(map[string]interface{})["email"]
		assert.False(t, hasEmail)
	}
	_, roster = aeList(t, f.app(aeTeacher, 1), fmt.Sprintf("/api/v1/courses/%d/enrollments?state[]=inactive", aeCourse))
	require.Len(t, roster, 1)
	assert.Equal(t, float64(aeStudentB), roster[0]["user_id"])
}

// ---------------------------------------------------------------------------
// G110 — multiple enrollments
// ---------------------------------------------------------------------------

func TestAdminEnrollments_MultipleEnrollmentsPerCourse(t *testing.T) {
	f := newAEFixture(t)
	teacher := f.app(aeTeacher, 1)
	create := func(body map[string]interface{}) (int, map[string]interface{}) {
		return aeDo(t, teacher, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse), map[string]interface{}{"enrollment": body})
	}
	// Bea (student in section B) becomes a TA in section A as well.
	code, body := create(map[string]interface{}{"user_id": aeStudentB, "type": "TaEnrollment", "course_section_id": aeSectionA})
	require.Equal(t, http.StatusCreated, code, body)
	// The exact same role in the same section is a duplicate.
	code, _ = create(map[string]interface{}{"user_id": aeStudentB, "type": "TaEnrollment", "course_section_id": aeSectionA})
	assert.Equal(t, http.StatusConflict, code)
	code, _ = create(map[string]interface{}{"user_id": aeStudentB, "type": "StudentEnrollment", "course_section_id": aeSectionB})
	assert.Equal(t, http.StatusConflict, code)

	// Permissions are the union: Bea passes the instructor gate.
	code, _ = aeDo(t, f.app(aeStudentB, 1), http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/teach", aeCourse), nil)
	assert.Equal(t, http.StatusOK, code)
	// Deactivating the TA enrollment leaves her a student only.
	var taRow models.Enrollment
	require.NoError(t, f.g.Where("user_id = ? AND course_id = ? AND type = ?", aeStudentB, aeCourse, "TaEnrollment").First(&taRow).Error)
	code, _ = aeDo(t, teacher, http.MethodDelete, enrollmentPath(aeCourse, taRow.ID)+"?task=deactivate", nil)
	require.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, f.app(aeStudentB, 1), http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/teach", aeCourse), nil)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = aeDo(t, f.app(aeStudentB, 1), http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse), nil)
	assert.Equal(t, http.StatusOK, code)
}

// ---------------------------------------------------------------------------
// G122 — invitations
// ---------------------------------------------------------------------------

func TestAdminEnrollments_Invitations(t *testing.T) {
	f := newAEFixture(t)
	teacher := f.app(aeTeacher, 1)
	outsider := f.app(aeOutsider, 1)
	modules := fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse)

	code, body := aeDo(t, teacher, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse), map[string]interface{}{
		"enrollment": map[string]interface{}{"user_id": aeOutsider, "type": "StudentEnrollment", "enrollment_state": "invited"},
	})
	require.Equal(t, http.StatusCreated, code, body)
	assert.Equal(t, "invited", body["enrollment_state"])
	invID := uint(body["id"].(float64))

	code, _ = aeDo(t, outsider, http.MethodGet, modules, nil)
	assert.Equal(t, http.StatusForbidden, code, "an invitee has no access until they accept")

	_, invites := aeList(t, outsider, "/api/v1/users/self/course_invitations")
	require.Len(t, invites, 1)
	assert.Equal(t, "Biology", invites[0]["course_name"])

	// Someone else can't accept it (404, no existence leak).
	code, _ = aeDo(t, f.app(aeStudentA, 1), http.MethodPost, enrollmentPath(aeCourse, invID)+"/accept", nil)
	assert.Equal(t, http.StatusNotFound, code)
	// Wrong course in the URL: 404.
	code, _ = aeDo(t, outsider, http.MethodPost, enrollmentPath(aeCourse2, invID)+"/accept", nil)
	assert.Equal(t, http.StatusNotFound, code)

	code, _ = aeDo(t, outsider, http.MethodPost, enrollmentPath(aeCourse, invID)+"/accept", nil)
	require.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, outsider, http.MethodPost, modules, nil)
	assert.Equal(t, http.StatusOK, code)
	_, invites = aeList(t, outsider, "/api/v1/users/self/course_invitations")
	assert.Len(t, invites, 0)
	// Accepting twice is refused.
	code, _ = aeDo(t, outsider, http.MethodPost, enrollmentPath(aeCourse, invID)+"/accept", nil)
	assert.Equal(t, http.StatusBadRequest, code)

	// Declining.
	code, body = aeDo(t, teacher, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse2), map[string]interface{}{
		"enrollment": map[string]interface{}{"user_id": aeOutsider, "type": "StudentEnrollment", "enrollment_state": "invited"},
	})
	require.Equal(t, http.StatusCreated, code, body)
	inv2 := uint(body["id"].(float64))
	code, _ = aeDo(t, outsider, http.MethodPost, enrollmentPath(aeCourse2, inv2)+"/reject", nil)
	require.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, outsider, http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse2), nil)
	assert.Equal(t, http.StatusForbidden, code)

	// Bad states are refused at creation.
	code, _ = aeDo(t, teacher, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enrollments", aeCourse), map[string]interface{}{
		"enrollment": map[string]interface{}{"user_id": aeOutsider, "type": "ObserverEnrollment", "enrollment_state": "completed"},
	})
	assert.Equal(t, http.StatusBadRequest, code)
}

// ---------------------------------------------------------------------------
// G108 / G109 — sections and cross-listing
// ---------------------------------------------------------------------------

func TestAdminEnrollments_SectionManagement(t *testing.T) {
	f := newAEFixture(t)
	teacher := f.app(aeTeacher, 1)
	sec := fmt.Sprintf("/api/v1/sections/%d", aeSectionB)

	code, body := aeDo(t, teacher, http.MethodPut, sec, map[string]interface{}{
		"course_section": map[string]interface{}{"name": "<b>Section</b> B (evening)", "start_at": "2026-01-10T00:00:00Z", "end_at": "2026-05-10T00:00:00Z", "restrict_enrollments_to_section_dates": true},
	})
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "Section B (evening)", body["name"], "plain-text name: markup stripped")
	assert.Equal(t, true, body["restrict_enrollments_to_section_dates"])

	// TAs don't manage sections; other tenants get a 404.
	code, _ = aeDo(t, f.app(aeTA, 1), http.MethodPut, sec, map[string]interface{}{"course_section": map[string]interface{}{"name": "x"}})
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = aeDo(t, f.app(aeTeacherT2, 2), http.MethodPut, sec, map[string]interface{}{"course_section": map[string]interface{}{"name": "x"}})
	assert.Equal(t, http.StatusNotFound, code)
	// Empty name / reversed dates.
	code, _ = aeDo(t, teacher, http.MethodPut, sec, map[string]interface{}{"course_section": map[string]interface{}{"name": "  "}})
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = aeDo(t, teacher, http.MethodPut, sec, map[string]interface{}{"course_section": map[string]interface{}{"end_at": "2025-01-01T00:00:00Z"}})
	assert.Equal(t, http.StatusBadRequest, code)

	// A section with people can't be deleted; an empty one can.
	code, _ = aeDo(t, teacher, http.MethodDelete, sec, nil)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = aeDo(t, teacher, http.MethodDelete, enrollmentPath(aeCourse, f.enrollments["studentB"])+"?task=delete", nil)
	require.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, teacher, http.MethodDelete, sec, nil)
	assert.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, teacher, http.MethodPut, sec, map[string]interface{}{"course_section": map[string]interface{}{"name": "back"}})
	assert.Equal(t, http.StatusNotFound, code, "a deleted section is gone")
}

func TestAdminEnrollments_Crosslisting(t *testing.T) {
	f := newAEFixture(t)
	teacher := f.app(aeTeacher, 1)
	xl := fmt.Sprintf("/api/v1/sections/%d/crosslist", aeSectionB)

	// Into another tenant's course: 404. Into a course the TA doesn't teach: 403.
	code, _ := aeDo(t, teacher, http.MethodPost, fmt.Sprintf("%s/%d", xl, aeCourseT2), nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = aeDo(t, f.app(aeTA, 1), http.MethodPost, fmt.Sprintf("%s/%d", xl, aeCourse2), nil)
	assert.Equal(t, http.StatusForbidden, code)

	code, body := aeDo(t, teacher, http.MethodPost, fmt.Sprintf("%s/%d", xl, aeCourse2), nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, float64(aeCourse2), body["course_id"])
	assert.Equal(t, float64(aeCourse), body["nonxlist_course_id"])

	var moved models.Enrollment
	require.NoError(t, f.g.First(&moved, f.enrollments["studentB"]).Error)
	assert.Equal(t, aeCourse2, moved.CourseID, "the section's enrollments follow it")
	code, _ = aeDo(t, f.app(aeStudentB, 1), http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse2), nil)
	assert.Equal(t, http.StatusOK, code)
	code, _ = aeDo(t, f.app(aeStudentB, 1), http.MethodGet, fmt.Sprintf("/api/v1/courses/%d/modules", aeCourse), nil)
	assert.Equal(t, http.StatusForbidden, code)

	// De-cross-list: back home.
	code, body = aeDo(t, teacher, http.MethodDelete, xl, nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, float64(aeCourse), body["course_id"])
	assert.Nil(t, body["nonxlist_course_id"])
	require.NoError(t, f.g.First(&moved, f.enrollments["studentB"]).Error)
	assert.Equal(t, aeCourse, moved.CourseID)
	code, _ = aeDo(t, teacher, http.MethodDelete, xl, nil)
	assert.Equal(t, http.StatusBadRequest, code, "not cross-listed any more")
}
