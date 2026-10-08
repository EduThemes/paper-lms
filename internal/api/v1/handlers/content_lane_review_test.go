package handlers_test

// Content-lane review follow-ups: gaps found while auditing the parity
// fixes (admin folder listing, folder ties, deleted-page revival).

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// B046 follow-up: a same-tenant admin (no enrollment) can still list a
// course folder's files; the new gate must not lock admins out.
func TestListFolderFiles_SameTenantAdminAllowed(t *testing.T) {
	h, folders, attachments, enrollments := newFileFixture(t)
	folders.On("FindByID", mock.Anything, uint(61), uint(1)).Return(&models.Folder{ID: 61, ContextType: "Course", ContextID: 99, WorkflowState: "visible"}, nil)
	enrollments.On("FindByUserAndCourse", mock.Anything, uint(7), uint(99), uint(1)).Return(nil, errNotFound).Maybe()
	attachments.On("ListByFolderID", mock.Anything, uint(61), mock.Anything).Return(&repository.PaginatedResult[models.Attachment]{Page: 1, PerPage: 10}, nil)

	app := testutil.SetupTestApp()
	app.Get("/folders/:folder_id/files", contentAuthStub(1, "", true), h.ListFolderFiles)

	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodGet, "/folders/61/files", nil).StatusCode)
}

func newFolderFixture(t *testing.T) (*handlers.FolderHandler, *mocks.MockFolderRepository) {
	folders := new(mocks.MockFolderRepository)
	attachments := new(mocks.MockAttachmentRepository)
	svc := service.NewFileService(folders, attachments, t.TempDir())
	return handlers.NewFolderHandler(svc, nil), folders
}

// A new course folder's parent must be a folder of the same course;
// otherwise a teacher could hang folders under another course's tree.
func TestCreateCourseFolder_ParentFromOtherCourse_404(t *testing.T) {
	h, folders := newFolderFixture(t)
	folders.On("FindByID", mock.Anything, uint(70), uint(1)).Return(&models.Folder{ID: 70, ContextType: "Course", ContextID: 99, FullName: "course files", WorkflowState: "visible"}, nil)
	folders.On("FindByID", mock.Anything, uint(71), uint(1)).Return(&models.Folder{ID: 71, ContextType: "Course", ContextID: 10, FullName: "course files", WorkflowState: "visible"}, nil)
	folders.On("Create", mock.Anything, mock.Anything).Return(nil)

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/folders", contentAuthStub(1, "TeacherEnrollment", false), h.CreateCourseFolder)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/10/folders", testutil.JSONBody(map[string]any{"name": "Week 2", "parent_folder_id": 70}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	folders.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/10/folders", testutil.JSONBody(map[string]any{"name": "Week 2", "parent_folder_id": 71}))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// Personal (User) folders can be renamed or deleted only by their owner
// (or a same-tenant admin); Group/Account folders only by an admin.
func TestUpdateDeleteFolder_NonCourseFolderOwnerOnly(t *testing.T) {
	h, folders := newFolderFixture(t)
	folders.On("FindByID", mock.Anything, uint(80), uint(1)).Return(&models.Folder{ID: 80, ContextType: "User", ContextID: 555, Name: "my files", WorkflowState: "visible"}, nil)
	folders.On("FindByID", mock.Anything, uint(81), uint(1)).Return(&models.Folder{ID: 81, ContextType: "User", ContextID: 7, Name: "my files", WorkflowState: "visible"}, nil)
	folders.On("FindByID", mock.Anything, uint(82), uint(1)).Return(&models.Folder{ID: 82, ContextType: "Group", ContextID: 3, Name: "group files", WorkflowState: "visible"}, nil)
	folders.On("Update", mock.Anything, mock.Anything).Return(nil)
	folders.On("Delete", mock.Anything, uint(81)).Return(nil)

	app := testutil.SetupTestApp()
	app.Put("/folders/:id", contentAuthStub(1, "", false), h.UpdateFolder)
	app.Delete("/folders/:id", contentAuthStub(1, "", false), h.DeleteFolder)

	rename := testutil.JSONBody(map[string]any{"name": "pwned"})
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodPut, "/folders/80", rename).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodDelete, "/folders/80", nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(app, http.MethodPut, "/folders/82", testutil.JSONBody(map[string]any{"name": "x"})).StatusCode)
	folders.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	folders.AssertNotCalled(t, "Delete", mock.Anything, uint(80))

	// The owner may.
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodPut, "/folders/81", testutil.JSONBody(map[string]any{"name": "renamed"})).StatusCode)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodDelete, "/folders/81", nil).StatusCode)

	admin := testutil.SetupTestApp()
	admin.Put("/folders/:id", contentAuthStub(1, "", true), h.UpdateFolder)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(admin, http.MethodPut, "/folders/82", testutil.JSONBody(map[string]any{"name": "y"})).StatusCode)
}

// B044 follow-up: a deleted page cannot be revived (and its slug
// duplicated) through the numeric PUT branch.
func TestUpdatePage_DeletedPageByNumericID_404(t *testing.T) {
	h, pages := newPageFixture()
	pages.On("FindByID", mock.Anything, uint(301), uint(1)).Return(&models.WikiPage{ID: 301, CourseID: 10, URL: "week-1", WorkflowState: "deleted", Title: "Week 1"}, nil)

	app := testutil.SetupTestApp()
	app.Put("/courses/:course_id/pages/:url_or_id", contentAuthStub(1, "TeacherEnrollment", false), h.UpdatePage)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/10/pages/301",
		testutil.JSONBody(map[string]any{"wiki_page": map[string]any{"published": true}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	pages.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}
