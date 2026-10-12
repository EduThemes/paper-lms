package handlers_test

// DB-backed tests for page history (G065) and page management rules
// (G074). All data is made up.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

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

// pfStub stamps the Locals the auth + course middleware set.
func pfStub(userID, accountID uint, enrollmentType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", accountID)
		if enrollmentType != "" {
			c.Locals("enrollment_type", enrollmentType)
		}
		return c.Next()
	}
}

type pageLaneFixture struct {
	g     *gorm.DB
	pages *handlers.PageHandler
	revs  *handlers.WikiPageRevisionHandler
}

func newPageLaneFixture(t *testing.T) *pageLaneFixture {
	t.Helper()
	g := coreScratchDB(t)
	for _, c := range []*models.Course{
		{ID: 100, AccountID: 1, Name: "Biology", CourseCode: "BIO", WorkflowState: models.CourseAvailable},
		{ID: 101, AccountID: 1, Name: "Chemistry", CourseCode: "CHEM", WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	pageRepo := postgres.NewPageRepository(g)
	revSvc := service.NewWikiPageRevisionService(postgres.NewWikiPageRevisionRepository(g), pageRepo)
	ph := handlers.NewPageHandler(service.NewPageService(pageRepo)).SetRevisionService(revSvc)
	return &pageLaneFixture{g: g, pages: ph, revs: handlers.NewWikiPageRevisionHandler(ph, revSvc)}
}

func (f *pageLaneFixture) app(userID uint, enrollmentType string) *fiber.App {
	app := testutil.SetupTestApp()
	app.Use(pfStub(userID, 1, enrollmentType))
	app.Post("/courses/:course_id/pages", f.pages.CreatePage)
	app.Get("/courses/:course_id/pages/:url_or_id", f.pages.GetPage)
	app.Put("/courses/:course_id/pages/:url_or_id", f.pages.UpdatePage)
	app.Delete("/courses/:course_id/pages/:url_or_id", f.pages.DeletePage)
	app.Get("/courses/:course_id/pages/:url_or_id/revisions", f.revs.ListRevisions)
	app.Get("/courses/:course_id/pages/:url_or_id/revisions/:revision_id", f.revs.GetRevision)
	app.Post("/courses/:course_id/pages/:url_or_id/revisions/:revision_id", f.revs.RevertToRevision)
	return app
}

func pageBody(fields map[string]any) io.Reader {
	return testutil.JSONBody(map[string]any{"wiki_page": fields})
}

func TestPageRevisions_RecordListAndRestore(t *testing.T) {
	f := newPageLaneFixture(t)
	teacher := f.app(8, "TeacherEnrollment")

	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Week 1", "body": "<p>first draft</p>", "published": true}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/week-1",
		pageBody(map[string]any{"body": "<p>second draft</p>"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// A save that changes nothing records nothing.
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/week-1",
		pageBody(map[string]any{"body": "<p>second draft</p>"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/week-1/revisions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	list, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	require.Len(t, list, 2)
	newest := list[0]
	assert.EqualValues(t, 2, newest["revision_id"])
	assert.Equal(t, true, newest["latest"])
	assert.Equal(t, "<p>second draft</p>", newest["body"])
	assert.EqualValues(t, 8, newest["edited_by"])

	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/week-1/revisions/1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	first, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "<p>first draft</p>", first["body"])
	assert.Equal(t, false, first["latest"])

	// Restore revision 1: the page body changes and history grows.
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages/week-1/revisions/1", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	restored, _ := testutil.ParseJSONMap(resp)
	assert.EqualValues(t, 3, restored["revision_id"])

	var page models.WikiPage
	require.NoError(t, f.g.Where("course_id = ? AND url = ?", 100, "week-1").First(&page).Error)
	assert.Equal(t, "<p>first draft</p>", page.Body)

	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/week-1/revisions/latest", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	latest, _ := testutil.ParseJSONMap(resp)
	assert.EqualValues(t, 3, latest["revision_id"])

	assert.Equal(t, http.StatusNotFound,
		testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages/week-1/revisions/99", nil).StatusCode)
}

// A page created outside the handler (import, module item) has no
// history; its first edit still records the pre-edit content.
func TestPageRevisions_FirstEditOfImportedPageKeepsBaseline(t *testing.T) {
	f := newPageLaneFixture(t)
	page := &models.WikiPage{CourseID: 100, Title: "Imported", Body: "<p>original</p>", WorkflowState: "active", EditingRoles: "teachers"}
	require.NoError(t, postgres.NewPageRepository(f.g).Create(context.Background(), page))

	teacher := f.app(8, "TeacherEnrollment")
	resp := testutil.MakeRequest(teacher, http.MethodPut, fmt.Sprintf("/courses/100/pages/%d", page.ID),
		pageBody(map[string]any{"body": "<p>edited</p>"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var revs []models.WikiPageRevision
	require.NoError(t, f.g.Where("wiki_page_id = ?", page.ID).Order("revision_number").Find(&revs).Error)
	require.Len(t, revs, 2)
	assert.Equal(t, "<p>original</p>", revs[0].Body)
	assert.Nil(t, revs[0].EditedBy)
	assert.Equal(t, "<p>edited</p>", revs[1].Body)
}

func TestPageEditingRoles_StudentEditRules(t *testing.T) {
	f := newPageLaneFixture(t)
	teacher := f.app(8, "TeacherEnrollment")
	student := f.app(9, "StudentEnrollment")

	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Class Notes", "body": "<p>x</p>", "published": true}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Teachers-only page: students can't edit or see history.
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"body": "<p>vandal</p>"})).StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodGet, "/courses/100/pages/class-notes/revisions", nil).StatusCode)
	resp = testutil.MakeRequest(student, http.MethodGet, "/courses/100/pages/class-notes", nil)
	got, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, false, got["can_edit"])

	// Invalid roles are rejected.
	assert.Equal(t, http.StatusBadRequest, testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"editing_roles": "teachers,janitors"})).StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"editing_roles": "teachers, students", "todo_date": "2026-11-02T15:00:00Z"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got, _ = testutil.ParseJSONMap(resp)
	assert.Equal(t, "teachers,students", got["editing_roles"])
	assert.Equal(t, "2026-11-02T15:00:00Z", got["todo_date"])

	// Now a student may change the content, but nothing else.
	resp = testutil.MakeRequest(student, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"body": "<p>student notes</p>"}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"published": false})).StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"editing_roles": "public"})).StatusCode)
	// Students with edit rights may read history but not restore it.
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(student, http.MethodGet, "/courses/100/pages/class-notes/revisions", nil).StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodPost, "/courses/100/pages/class-notes/revisions/1", nil).StatusCode)

	// Clearing the to-do date.
	resp = testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"todo_date": nil}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got, _ = testutil.ParseJSONMap(resp)
	assert.Nil(t, got["todo_date"])

	// Unpublished: the page is gone for students, edit rights or not.
	require.Equal(t, http.StatusOK, testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"published": false})).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(student, http.MethodPut, "/courses/100/pages/class-notes",
		pageBody(map[string]any{"body": "<p>late edit</p>"})).StatusCode)
}

func TestPages_FrontPageRulesAndParentTie(t *testing.T) {
	f := newPageLaneFixture(t)
	teacher := f.app(8, "TeacherEnrollment")

	assert.Equal(t, http.StatusBadRequest, testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Home", "front_page": true, "published": false})).StatusCode)

	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Home", "front_page": true, "published": true}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	home, _ := testutil.ParseJSONMap(resp)

	assert.Equal(t, http.StatusBadRequest, testutil.MakeRequest(teacher, http.MethodPut, "/courses/100/pages/home",
		pageBody(map[string]any{"published": false})).StatusCode)
	assert.Equal(t, http.StatusBadRequest, testutil.MakeRequest(teacher, http.MethodDelete, "/courses/100/pages/home", nil).StatusCode)

	// The numeric id of a course-100 page is a 404 under course 101.
	path := fmt.Sprintf("/courses/101/pages/%v/revisions", home["page_id"])
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(teacher, http.MethodGet, path, nil).StatusCode)

	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/100/pages",
		pageBody(map[string]any{"title": "Scratch", "published": false}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(teacher, http.MethodDelete, "/courses/100/pages/scratch", nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(teacher, http.MethodGet, "/courses/100/pages/scratch", nil).StatusCode)
}
