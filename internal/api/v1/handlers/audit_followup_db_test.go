package handlers_test

// DB-backed regression tests for the post-merge authorization sweep:
// annotations (F0/F1), accommodations (F2), GET /courses?scope=all (F3),
// group memberships and group context (F6/F7), and the FERPA/COPPA admin
// routes keyed by bare ids (F8). All data is made up.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// Fixture ids. Tenant 1: course 100 (teacher 20, students 21 and 22),
// course 101 (no enrollments), admin 31. Tenant 2: course 200 (teacher 30),
// admin 3 (seeded by coreScratchDB).
const (
	fxTeacherA  = uint(20)
	fxStudent   = uint(21)
	fxClassmate = uint(22)
	fxTeacherB  = uint(30)
	fxAdminA    = uint(31)
	fxAdminB    = uint(3)
	fxCourseA   = uint(100)
	fxCourseA2  = uint(101)
	fxCourseB   = uint(200)
	fxAssign    = uint(1000)
)

type followupFixture struct {
	g          *gorm.DB
	authz      *handlers.ResourceAuthorizer
	submission *models.Submission
	annotation *models.DocumentAnnotation
}

func newFollowupFixture(t *testing.T) *followupFixture {
	t.Helper()
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: fxTeacherA, AccountID: 1, Name: "Terry Teacher", LoginID: "terry@example.test", Email: "terry@example.test", PasswordHash: "x", Role: "user"},
		{ID: fxStudent, AccountID: 1, Name: "Sam Student", LoginID: "sam@example.test", Email: "sam@example.test", PasswordHash: "x", Role: "user"},
		{ID: fxClassmate, AccountID: 1, Name: "Chris Classmate", LoginID: "chris@example.test", Email: "chris@example.test", PasswordHash: "x", Role: "user"},
		{ID: fxTeacherB, AccountID: 2, Name: "Tara Teacher", LoginID: "tara@example.test", Email: "tara@example.test", PasswordHash: "x", Role: "user"},
		{ID: fxAdminA, AccountID: 1, Name: "Alex Admin", LoginID: "alex@example.test", Email: "alex@example.test", PasswordHash: "x", Role: "admin"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	for _, c := range []*models.Course{
		{ID: fxCourseA, AccountID: 1, Name: "Biology", CourseCode: "BIO", WorkflowState: models.CourseAvailable},
		{ID: fxCourseA2, AccountID: 1, Name: "Draft Chemistry", CourseCode: "CHEM", WorkflowState: models.CourseUnpublished},
		{ID: fxCourseB, AccountID: 2, Name: "History", CourseCode: "HIST", WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	for _, e := range []*models.Enrollment{
		{UserID: fxTeacherA, CourseID: fxCourseA, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: fxStudent, CourseID: fxCourseA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: fxClassmate, CourseID: fxCourseA, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: fxTeacherB, CourseID: fxCourseB, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
	}
	require.NoError(t, g.Create(&models.Assignment{ID: fxAssign, CourseID: fxCourseA, Name: "Essay", WorkflowState: models.AssignmentPublished}).Error)
	sub := &models.Submission{AssignmentID: fxAssign, UserID: fxStudent, WorkflowState: models.SubmissionSubmitted}
	require.NoError(t, g.Create(sub).Error)
	ann := &models.DocumentAnnotation{SubmissionID: sub.ID, UserID: fxTeacherA, AnnotationType: "comment", Content: "Cite your sources", PageNumber: 1, Color: "#FFFF00", WorkflowState: "active"}
	require.NoError(t, g.Create(ann).Error)

	authz := handlers.NewResourceAuthorizer(postgres.NewEnrollmentRepository(g), users, postgres.NewCourseRepository(g))
	return &followupFixture{g: g, authz: authz, submission: sub, annotation: ann}
}

func (f *followupFixture) annotationApp(userID, accountID uint) *fiber.App {
	g := f.g
	submissionRepo := postgres.NewSubmissionRepository(g)
	assignmentRepo := postgres.NewAssignmentRepository(g)
	enrollmentRepo := postgres.NewEnrollmentRepository(g)
	courseRepo := postgres.NewCourseRepository(g)
	submissionService := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo,
		postgres.NewLatePolicyRepository(g), courseRepo, postgres.NewGradingPeriodGroupRepository(g),
		postgres.NewGradingPeriodRepository(g), postgres.NewGroupMembershipRepository(g))
	annService := service.NewDocumentAnnotationService(postgres.NewDocumentAnnotationRepository(g), submissionRepo, postgres.NewAttachmentRepository(g), enrollmentRepo)
	h := handlers.NewDocumentAnnotationHandler(annService, submissionService, assignmentRepo, submissionRepo, f.authz)

	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	base := "/courses/:course_id/assignments/:assignment_id/submissions/:user_id"
	app.Get(base+"/annotations", h.ListAnnotations)
	app.Post(base+"/annotations", h.CreateAnnotation)
	app.Get(base+"/annotation_summary", h.GetAnnotationSummary)
	app.Get("/annotations/:id", h.GetAnnotation)
	app.Delete("/annotations/:id", h.DeleteAnnotation)
	app.Post("/annotations/:id/resolve", h.ResolveAnnotation)
	app.Post("/annotations/:id/replies", h.ReplyToAnnotation)
	return app
}

func (f *followupFixture) annotationState(t *testing.T) string {
	t.Helper()
	var a models.DocumentAnnotation
	require.NoError(t, f.g.First(&a, f.annotation.ID).Error)
	return a.WorkflowState
}

func (f *followupFixture) replyCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.g.Model(&models.DocumentAnnotation{}).Where("parent_annotation_id = ?", f.annotation.ID).Count(&n).Error)
	return n
}

// F0: a teacher of another tenant's course can't delete or reply to an
// annotation by passing their own course_id.
func TestAnnotations_CrossTenantDeleteAndReplyAreRefused(t *testing.T) {
	f := newFollowupFixture(t)
	app := f.annotationApp(fxTeacherB, 2)

	resp := testutil.MakeRequest(app, http.MethodDelete, fmt.Sprintf("/annotations/%d?course_id=%d", f.annotation.ID, fxCourseB), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "active", f.annotationState(t))

	body := testutil.JSONBody(map[string]interface{}{"annotation": map[string]interface{}{"content": "planted"}})
	resp = testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/annotations/%d/replies?course_id=%d", f.annotation.ID, fxCourseB), body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, int64(0), f.replyCount(t))
}

// F1: a classmate can't read, resolve, reply to or delete the instructor's
// feedback on another student's submission.
func TestAnnotations_ClassmateCannotReadOrResolve(t *testing.T) {
	f := newFollowupFixture(t)
	app := f.annotationApp(fxClassmate, 1)
	list := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/annotations", fxCourseA, fxAssign, fxStudent)

	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, list, nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet,
		fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/annotation_summary", fxCourseA, fxAssign, fxStudent), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/annotations/%d", f.annotation.ID), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodPost, fmt.Sprintf("/annotations/%d/resolve", f.annotation.ID), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodDelete, fmt.Sprintf("/annotations/%d?course_id=%d", f.annotation.ID, fxCourseA), nil).StatusCode)
	assert.Equal(t, "active", f.annotationState(t))

	// The owner and the teacher can read it.
	owner := f.annotationApp(fxStudent, 1)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(owner, http.MethodGet, list, nil).StatusCode)
	teacher := f.annotationApp(fxTeacherA, 1)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(teacher, http.MethodGet, list, nil).StatusCode)
	// The teacher can delete it, without passing any course_id.
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(teacher, http.MethodDelete, fmt.Sprintf("/annotations/%d", f.annotation.ID), nil).StatusCode)
	assert.Equal(t, "deleted", f.annotationState(t))
}

// F1: the URL's course must own the assignment.
func TestAnnotations_AssignmentMustBelongToURLCourse(t *testing.T) {
	f := newFollowupFixture(t)
	app := f.annotationApp(fxStudent, 1)
	wrong := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d/annotations", fxCourseA2, fxAssign, fxStudent)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodGet, wrong, nil).StatusCode)
}

func accommodationApp(f *followupFixture, userID, accountID uint) *fiber.App {
	svc := service.NewAccommodationService(postgres.NewStudentAccommodationRepository(f.g), postgres.NewAccommodationApplicationRepository(f.g))
	h := handlers.NewAccommodationHandler(svc, nil, f.authz, nil)
	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	app.Post("/users/:user_id/accommodations", h.CreateAccommodation)
	app.Put("/accommodations/:id", h.UpdateAccommodation)
	app.Delete("/accommodations/:id", h.DeleteAccommodation)
	return app
}

// F2: an admin of tenant 2 can't create, change or deactivate a tenant-1
// student's accommodation; the creator is always the caller.
func TestAccommodations_CrossTenantAdminIsRefused(t *testing.T) {
	f := newFollowupFixture(t)
	acc := &models.StudentAccommodation{UserID: fxStudent, AccommodationType: "extended_time", Status: "active", CreatedByID: fxAdminA, EffectiveFrom: time.Now().Add(-time.Hour)}
	require.NoError(t, f.g.Create(acc).Error)

	foreign := accommodationApp(f, fxAdminB, 2)
	body := func() io.Reader {
		return testutil.JSONBody(map[string]interface{}{"accommodation": map[string]interface{}{"accommodation_type": "extended_time", "created_by_id": 999}})
	}
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodPost, fmt.Sprintf("/users/%d/accommodations", fxStudent), body()).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodPut, fmt.Sprintf("/accommodations/%d", acc.ID),
		testutil.JSONBody(map[string]interface{}{"accommodation": map[string]interface{}{"status": "inactive"}})).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodDelete, fmt.Sprintf("/accommodations/%d", acc.ID), nil).StatusCode)
	var after models.StudentAccommodation
	require.NoError(t, f.g.First(&after, acc.ID).Error)
	assert.Equal(t, "active", after.Status)

	own := accommodationApp(f, fxAdminA, 1)
	resp := testutil.MakeRequest(own, http.MethodPost, fmt.Sprintf("/users/%d/accommodations", fxStudent), body())
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.EqualValues(t, fxAdminA, created["created_by_id"])
}

// F3: ?scope=all lists the whole tenant only for admins.
func TestListCourses_ScopeAllIsAdminOnly(t *testing.T) {
	f := newFollowupFixture(t)
	g := f.g
	courseRepo := postgres.NewCourseRepository(g)
	enrollmentRepo := postgres.NewEnrollmentRepository(g)
	h := handlers.NewCourseHandler(service.NewCourseService(courseRepo, enrollmentRepo, postgres.NewSectionRepository(g)), service.NewEnrollmentService(enrollmentRepo)).
		SetUserRepo(postgres.NewUserRepository(g))
	ids := func(userID, accountID uint) []uint {
		app := testutil.SetupTestApp()
		app.Use(callerStub(userID, accountID))
		app.Get("/courses", h.ListCourses)
		resp := testutil.MakeRequest(app, http.MethodGet, "/courses?scope=all&per_page=50", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var rows []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&rows))
		out := []uint{}
		for _, r := range rows {
			out = append(out, uint(r["id"].(float64)))
		}
		return out
	}
	assert.ElementsMatch(t, []uint{fxCourseA}, ids(fxStudent, 1))
	assert.ElementsMatch(t, []uint{fxCourseA, fxCourseA2}, ids(fxAdminA, 1))
}

func groupApp(f *followupFixture, userID, accountID uint) *fiber.App {
	g := f.g
	svc := service.NewGroupService(postgres.NewGroupCategoryRepository(g), postgres.NewGroupRepository(g), postgres.NewGroupMembershipRepository(g), postgres.NewEnrollmentRepository(g))
	h := handlers.NewGroupHandler(svc, f.authz, nil)
	app := testutil.SetupTestApp()
	app.Use(callerStub(userID, accountID))
	app.Post("/group_categories/:group_category_id/groups", h.CreateGroup)
	app.Get("/groups/:id", h.GetGroup)
	app.Put("/groups/:id", h.UpdateGroup)
	app.Delete("/groups/:id", h.DeleteGroup)
	app.Get("/groups/:group_id/memberships", h.ListGroupMemberships)
	return app
}

func seedCourseGroup(t *testing.T, g *gorm.DB) (*models.GroupCategory, *models.Group) {
	t.Helper()
	cid := fxCourseA
	cat := &models.GroupCategory{CourseID: &cid, Name: "Lab partners"}
	require.NoError(t, g.Create(cat).Error)
	grp := &models.Group{GroupCategoryID: cat.ID, Name: "Team 1", ContextType: "Course", ContextID: fxCourseA}
	require.NoError(t, g.Create(grp).Error)
	for _, uid := range []uint{fxStudent, fxClassmate} {
		require.NoError(t, g.Create(&models.GroupMembership{GroupID: grp.ID, UserID: uid, WorkflowState: "accepted"}).Error)
	}
	return cat, grp
}

func membershipEmails(t *testing.T, app *fiber.App, groupID uint) []interface{} {
	t.Helper()
	resp := testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/groups/%d/memberships", groupID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var rows []map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rows))
	out := []interface{}{}
	for _, r := range rows {
		if u, ok := r["user"].(map[string]interface{}); ok {
			if e, ok := u["email"]; ok {
				out = append(out, e)
			}
		}
	}
	return out
}

// F6: classmates see member names but not email/login_id; staff do.
func TestGroupMemberships_ContactFieldsAreStaffOnly(t *testing.T) {
	f := newFollowupFixture(t)
	_, grp := seedCourseGroup(t, f.g)
	assert.Empty(t, membershipEmails(t, groupApp(f, fxClassmate, 1), grp.ID))
	assert.Len(t, membershipEmails(t, groupApp(f, fxTeacherA, 1), grp.ID), 2)
}

// F7: a tenant-2 caller can't read, rename or delete a tenant-1 group, and
// CreateGroup derives the context from the category, not the body.
func TestGroups_CrossTenantAndClientContextAreRefused(t *testing.T) {
	f := newFollowupFixture(t)
	cat, grp := seedCourseGroup(t, f.g)
	foreign := groupApp(f, fxAdminB, 2)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodGet, fmt.Sprintf("/groups/%d", grp.ID), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodPut, fmt.Sprintf("/groups/%d", grp.ID),
		testutil.JSONBody(map[string]interface{}{"group": map[string]interface{}{"name": "Renamed"}})).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodDelete, fmt.Sprintf("/groups/%d", grp.ID), nil).StatusCode)

	teacher := groupApp(f, fxTeacherA, 1)
	resp := testutil.MakeRequest(teacher, http.MethodPost, fmt.Sprintf("/group_categories/%d/groups", cat.ID),
		testutil.JSONBody(map[string]interface{}{"group": map[string]interface{}{"name": "Planted", "context_type": "Account", "context_id": 2}}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created models.Group
	require.NoError(t, f.g.Where("name = ?", "Planted").First(&created).Error)
	assert.Equal(t, "Course", created.ContextType)
	assert.Equal(t, fxCourseA, created.ContextID)
}

// F8: tenant-2 admins can't read a tenant-1 user's PII access log, revoke
// a tenant-1 student's parental consent, or edit a tenant-1 DPA.
func TestFERPACOPPA_CrossTenantAdminIsRefused(t *testing.T) {
	f := newFollowupFixture(t)
	g := f.g
	require.NoError(t, g.Create(&models.PIIAccessLog{AccessorID: fxTeacherA, StudentID: fxStudent, AccessType: "view", DataField: "email", IPAddress: "192.0.2.1"}).Error)
	consent := &models.ParentalConsent{StudentID: fxStudent, ParentName: "Pat Parent", ParentEmail: "pat@example.test", ConsentType: "data_collection", Status: "granted"}
	require.NoError(t, g.Create(consent).Error)
	dpa := &models.DataProcessingAgreement{AccountID: 1, VendorName: "Example Vendor", Purpose: "Testing"}
	require.NoError(t, g.Create(dpa).Error)

	ferpa := handlers.NewFERPAHandler(service.NewFERPAService(postgres.NewDataRetentionPolicyRepository(g), postgres.NewDataDeletionRequestRepository(g),
		postgres.NewDataExportRequestRepository(g), postgres.NewPIIAccessLogRepository(g))).SetAuthorizer(f.authz)
	coppa := handlers.NewCOPPAHandler(service.NewCOPPAService(postgres.NewParentalConsentRepository(g), postgres.NewDataProcessingAgreementRepository(g),
		postgres.NewAgeVerificationRepository(g))).SetAuthorizer(f.authz)
	mk := func(userID, accountID uint) *fiber.App {
		app := testutil.SetupTestApp()
		app.Use(callerStub(userID, accountID))
		app.Get("/users/:user_id/pii_access_log", ferpa.GetPIIAccessLog)
		app.Delete("/consent/:id", coppa.RevokeConsent)
		app.Put("/data_processing_agreements/:id", coppa.UpdateDPA)
		return app
	}
	foreign := mk(fxAdminB, 2)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodGet, fmt.Sprintf("/users/%d/pii_access_log", fxStudent), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodDelete, fmt.Sprintf("/consent/%d", consent.ID), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(foreign, http.MethodPut, fmt.Sprintf("/data_processing_agreements/%d", dpa.ID),
		testutil.JSONBody(map[string]interface{}{"vendor_name": "Hijacked"})).StatusCode)

	var c models.ParentalConsent
	require.NoError(t, g.First(&c, consent.ID).Error)
	assert.Equal(t, "granted", c.Status)
	var d models.DataProcessingAgreement
	require.NoError(t, g.First(&d, dpa.ID).Error)
	assert.Equal(t, "Example Vendor", d.VendorName)

	own := mk(fxAdminA, 1)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(own, http.MethodGet, fmt.Sprintf("/users/%d/pii_access_log", fxStudent), nil).StatusCode)
}

// F16: a user who never picked a language has no profile locale (''), so
// the client falls back to the device choice and the account default
// instead of a column default of 'en' that always won.
func TestUsers_NoExplicitLocaleIsEmpty(t *testing.T) {
	f := newFollowupFixture(t)
	var u models.User
	require.NoError(t, f.g.First(&u, fxStudent).Error)
	assert.Equal(t, "", u.Locale)

	u.Locale = "es"
	require.NoError(t, f.g.Save(&u).Error)
	var again models.User
	require.NoError(t, f.g.First(&again, fxStudent).Error)
	assert.Equal(t, "es", again.Locale)
}

// Leftover (core): events with no acting user (failed login for an unknown
// account) must be stored with user_id NULL instead of failing the
// audit_logs.user_id FK and being dropped.
func TestAuditLog_SystemEventWithoutUserIsStored(t *testing.T) {
	f := newFollowupFixture(t)
	repo := postgres.NewAuditLogRepository(f.g)
	svc := service.NewAuditService(repo, nil, nil)
	require.NoError(t, svc.LogEvent(context.Background(), "auth.login_failed", 0, nil, nil, "User", 0, "login_failed", `{"reason":"unknown_user"}`, "192.0.2.10", "test-agent"))
	var n int64
	require.NoError(t, f.g.Model(&models.AuditLog{}).Where("event_type = ? AND user_id IS NULL", "auth.login_failed").Count(&n).Error)
	assert.Equal(t, int64(1), n)
	var row models.AuditLog
	require.NoError(t, f.g.Where("event_type = ?", "auth.login_failed").First(&row).Error, "a NULL user_id row must still load")
	assert.Equal(t, uint(0), row.UserID)
}
