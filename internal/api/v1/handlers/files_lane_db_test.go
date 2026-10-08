package handlers_test

// DB-backed tests for file visibility (G070), file management (G071) and
// course storage quotas (G072). All data is made up.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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
	flTeacher = uint(40)
	flStudent = uint(41)
	flAdmin   = uint(42)
	flCourse  = uint(100)
	flOther   = uint(101)
)

type filesLaneFixture struct {
	g       *gorm.DB
	files   *handlers.FileHandler
	folders *handlers.FolderHandler
	courses *handlers.CourseHandler
}

func newFilesLaneFixture(t *testing.T) *filesLaneFixture {
	t.Helper()
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: flTeacher, AccountID: 1, Name: "Terry Teacher", LoginID: "terry.fl@example.test", Email: "terry.fl@example.test", PasswordHash: "x", Role: "user"},
		{ID: flStudent, AccountID: 1, Name: "Sam Student", LoginID: "sam.fl@example.test", Email: "sam.fl@example.test", PasswordHash: "x", Role: "user"},
		{ID: flAdmin, AccountID: 1, Name: "Alex Admin", LoginID: "alex.fl@example.test", Email: "alex.fl@example.test", PasswordHash: "x", Role: "admin"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	for _, c := range []*models.Course{
		{ID: flCourse, AccountID: 1, Name: "Biology", CourseCode: "BIO", WorkflowState: models.CourseAvailable},
		{ID: flOther, AccountID: 1, Name: "Chemistry", CourseCode: "CHEM", WorkflowState: models.CourseAvailable},
	} {
		require.NoError(t, g.Create(c).Error)
	}
	for _, e := range []*models.Enrollment{
		{UserID: flTeacher, CourseID: flCourse, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: flStudent, CourseID: flCourse, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: flTeacher, CourseID: flOther, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
	}

	enrollRepo := postgres.NewEnrollmentRepository(g)
	courseRepo := postgres.NewCourseRepository(g)
	authz := handlers.NewResourceAuthorizer(enrollRepo, users, courseRepo)
	fileSvc := service.NewFileService(postgres.NewFolderRepository(g), postgres.NewAttachmentRepository(g), t.TempDir()).
		SetTreeRepository(postgres.NewCourseFilesRepository(g))
	fh := handlers.NewFileHandler(fileSvc, enrollRepo, nil, nil).SetAuthorizer(authz).
		SetQuota(service.NewStorageQuotaService(g))
	ch := handlers.NewCourseHandler(service.NewCourseService(courseRepo, enrollRepo, nil), service.NewEnrollmentService(enrollRepo)).SetUserRepo(users).
		SetStorageQuotas(service.NewStorageQuotaService(g))
	return &filesLaneFixture{g: g, files: fh, folders: handlers.NewFolderHandler(fileSvc, authz), courses: ch}
}

// app mounts the routes the way router.go does: course routes get the
// enrollment type from the permission middleware; /files and /folders
// routes don't.
func (f *filesLaneFixture) app(userID uint, enrollmentType string) *fiber.App {
	app := testutil.SetupTestApp()
	course := pfStub(userID, 1, enrollmentType)
	bare := pfStub(userID, 1, "")
	app.Get("/courses/:course_id/files/quota", course, f.files.GetCourseQuota)
	app.Get("/courses/:course_id/files", course, f.files.ListCourseFiles)
	app.Post("/courses/:course_id/files", course, f.files.UploadCourseFile)
	app.Get("/courses/:course_id/files/:id", course, f.files.GetFile)
	app.Put("/courses/:course_id/files/:id", course, f.files.UpdateCourseFile)
	app.Post("/courses/:course_id/files/:id/replace", course, f.files.ReplaceCourseFile)
	app.Get("/courses/:course_id/folders", course, f.folders.ListCourseFolders)
	app.Post("/courses/:course_id/folders", course, f.folders.CreateCourseFolder)
	app.Put("/courses/:id", course, f.courses.UpdateCourse)
	app.Get("/files/:id/download", bare, f.files.DownloadFile)
	app.Get("/folders/:folder_id/files", bare, f.files.ListFolderFiles)
	app.Get("/folders/:id", bare, f.folders.GetFolder)
	app.Put("/folders/:id", bare, f.folders.UpdateFolder)
	app.Delete("/folders/:id", bare, f.folders.DeleteFolder)
	return app
}

func flUpload(t *testing.T, app *fiber.App, courseID uint, name, content string, fields map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, name))
	hdr.Set("Content-Type", "text/plain")
	fw, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = fw.Write([]byte(content))
	require.NoError(t, w.Close())
	path := fmt.Sprintf("/courses/%d/files", courseID)
	if id, ok := fields["_replace"]; ok {
		path = fmt.Sprintf("/courses/%d/files/%s/replace", courseID, id)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	body, _ := testutil.ParseJSONMap(resp)
	return resp, body
}

func flJSON(t *testing.T, app *fiber.App, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = testutil.JSONBody(body)
	}
	resp := testutil.MakeRequest(app, method, path, r)
	out, _ := testutil.ParseJSONMap(resp)
	return resp, out
}

func flNames(t *testing.T, resp *http.Response) []string {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	items, err := testutil.ParseJSONArray(resp)
	require.NoError(t, err)
	names := make([]string, 0, len(items))
	for _, it := range items {
		if n, ok := it["display_name"].(string); ok {
			names = append(names, n)
		} else {
			names = append(names, it["name"].(string))
		}
	}
	return names
}

func TestFileVisibility_LearnersAndStaff(t *testing.T) {
	f := newFilesLaneFixture(t)
	teacher := f.app(flTeacher, "TeacherEnrollment")
	student := f.app(flStudent, "StudentEnrollment")

	_, pub := flUpload(t, teacher, flCourse, "syllabus.txt", "public", nil)
	_, key := flUpload(t, teacher, flCourse, "answer-key.txt", "secret", nil)
	_, hid := flUpload(t, teacher, flCourse, "bonus.txt", "by link", nil)
	_, later := flUpload(t, teacher, flCourse, "week-9.txt", "not yet", nil)
	resp, folder := flJSON(t, teacher, http.MethodPost, fmt.Sprintf("/courses/%d/folders", flCourse), map[string]any{"name": "Exams"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	_, inFolder := flUpload(t, teacher, flCourse, "midterm.txt", "exam", map[string]string{"folder_id": fmt.Sprint(folder["id"])})

	put := func(id any, body map[string]any) {
		resp, _ := flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/courses/%d/files/%v", flCourse, id), body)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	put(key["id"], map[string]any{"locked": true})
	put(hid["id"], map[string]any{"hidden": true})
	put(later["id"], map[string]any{"unlock_at": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)})
	resp, _ = flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/folders/%v", folder["id"]), map[string]any{"locked": true})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Staff see everything; learners only published, unhidden files
	// outside unpublished folders.
	all := fmt.Sprintf("/courses/%d/files", flCourse)
	assert.ElementsMatch(t, []string{"answer-key.txt", "bonus.txt", "midterm.txt", "syllabus.txt", "week-9.txt"},
		flNames(t, testutil.MakeRequest(teacher, http.MethodGet, all, nil)))
	assert.ElementsMatch(t, []string{"syllabus.txt", "week-9.txt"},
		flNames(t, testutil.MakeRequest(student, http.MethodGet, all, nil)))
	assert.Empty(t, flNames(t, testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/courses/%d/folders?parent_folder_id=root", flCourse), nil)))

	file := func(app *fiber.App, id any) int {
		return testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/courses/%d/files/%v", flCourse, id), nil).StatusCode
	}
	download := func(app *fiber.App, id any) int {
		return testutil.MakeRequest(app, http.MethodGet, fmt.Sprintf("/files/%v/download", id), nil).StatusCode
	}
	assert.Equal(t, http.StatusNotFound, file(student, key["id"]))
	assert.Equal(t, http.StatusNotFound, download(student, key["id"]))
	assert.Equal(t, http.StatusOK, file(student, hid["id"]), "hidden files stay readable by link")
	assert.Equal(t, http.StatusOK, download(student, hid["id"]))
	assert.Equal(t, http.StatusForbidden, download(student, later["id"]))
	resp, meta := flJSON(t, student, http.MethodGet, fmt.Sprintf("/courses/%d/files/%v", flCourse, later["id"]), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, meta["locked_for_user"])
	assert.Equal(t, http.StatusNotFound, download(student, inFolder["id"]))
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/folders/%v/files", folder["id"]), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/folders/%v", folder["id"]), nil).StatusCode)
	assert.Equal(t, http.StatusOK, download(student, pub["id"]))

	// Staff reach everything, including via the bare download route.
	for _, id := range []any{key["id"], hid["id"], later["id"], inFolder["id"]} {
		assert.Equal(t, http.StatusOK, download(teacher, id))
	}

	// Usage rights are validated and stored.
	resp, _ = flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/courses/%d/files/%v", flCourse, pub["id"]),
		map[string]any{"usage_rights": map[string]any{"use_justification": "stolen"}})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp, out := flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/courses/%d/files/%v", flCourse, pub["id"]),
		map[string]any{"usage_rights": map[string]any{"use_justification": "creative_commons", "legal_copyright": "Jane Doe", "license": "cc_by"}})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "creative_commons", out["usage_rights"].(map[string]any)["use_justification"])
}

func TestFileManagement_RenameMoveReplaceAndFolders(t *testing.T) {
	f := newFilesLaneFixture(t)
	teacher := f.app(flTeacher, "TeacherEnrollment")

	_, a := flUpload(t, teacher, flCourse, "notes.txt", "v1", nil)
	_, b := flUpload(t, teacher, flCourse, "other.txt", "x", nil)
	_, units := flJSON(t, teacher, http.MethodPost, fmt.Sprintf("/courses/%d/folders", flCourse), map[string]any{"name": "Units"})
	_, unit1 := flJSON(t, teacher, http.MethodPost, fmt.Sprintf("/courses/%d/folders", flCourse), map[string]any{"name": "Unit 1", "parent_folder_id": units["id"]})
	_, foreign := flJSON(t, f.app(flTeacher, "TeacherEnrollment"), http.MethodPost, fmt.Sprintf("/courses/%d/folders", flOther), map[string]any{"name": "Chem"})

	path := func(id any) string { return fmt.Sprintf("/courses/%d/files/%v", flCourse, id) }

	// Rename; a clash is a 409 unless on_duplicate=rename.
	resp, _ := flJSON(t, teacher, http.MethodPut, path(b["id"]), map[string]any{"display_name": "notes.txt"})
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	resp, out := flJSON(t, teacher, http.MethodPut, path(b["id"]), map[string]any{"display_name": "notes.txt", "on_duplicate": "rename"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "notes (1).txt", out["display_name"])

	// Move into a folder of this course; another course's folder is a 404.
	resp, _ = flJSON(t, teacher, http.MethodPut, path(a["id"]), map[string]any{"parent_folder_id": foreign["id"]})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp, out = flJSON(t, teacher, http.MethodPut, path(a["id"]), map[string]any{"parent_folder_id": unit1["id"]})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, unit1["id"], out["folder_id"])
	assert.Equal(t, []string{"notes.txt"}, flNames(t, testutil.MakeRequest(teacher, http.MethodGet,
		fmt.Sprintf("/courses/%d/files?folder_id=%v", flCourse, unit1["id"]), nil)))
	assert.Equal(t, []string{"notes (1).txt"}, flNames(t, testutil.MakeRequest(teacher, http.MethodGet,
		fmt.Sprintf("/courses/%d/files?folder_id=root", flCourse), nil)))

	// Replace keeps the id and serves the new bytes.
	resp, out = flUpload(t, teacher, flCourse, "notes-v2.txt", "version two", map[string]string{"_replace": fmt.Sprint(a["id"])})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, a["id"], out["id"])
	assert.Equal(t, "notes.txt", out["display_name"])
	dl := testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/files/%v/download", a["id"]), nil)
	got, _ := io.ReadAll(dl.Body)
	assert.Equal(t, "version two", string(got))

	// Renaming a folder refreshes the full path of its subtree.
	resp, _ = flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/folders/%v", units["id"]), map[string]any{"name": "Modules"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var child models.Folder
	require.NoError(t, f.g.First(&child, unit1["id"]).Error)
	assert.Equal(t, "course files/Modules/Unit 1", child.FullName)

	// A folder can't move under itself.
	resp, _ = flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/folders/%v", units["id"]), map[string]any{"parent_folder_id": unit1["id"]})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	// Moving Unit 1 to the root works.
	resp, out = flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/folders/%v", unit1["id"]), map[string]any{"parent_folder_id": "root"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "course files/Unit 1", out["full_name"])
	assert.ElementsMatch(t, []string{"Modules", "Unit 1"}, flNames(t, testutil.MakeRequest(teacher, http.MethodGet,
		fmt.Sprintf("/courses/%d/folders?parent_folder_id=root", flCourse), nil)))

	// Deleting a folder deletes what's in it.
	resp, _ = flJSON(t, teacher, http.MethodDelete, fmt.Sprintf("/folders/%v", unit1["id"]), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(teacher, http.MethodGet, path(a["id"]), nil).StatusCode)
}

func TestCourseStorageQuota(t *testing.T) {
	f := newFilesLaneFixture(t)
	teacher := f.app(flTeacher, "TeacherEnrollment")
	admin := f.app(flAdmin, "")

	// Teachers can't change the quota; admins can.
	resp, _ := flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/courses/%d", flCourse), map[string]any{"course": map[string]any{"storage_quota_mb": 1}})
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp, out := flJSON(t, admin, http.MethodPut, fmt.Sprintf("/courses/%d", flCourse), map[string]any{"course": map[string]any{"storage_quota_mb": 1}})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 1, out["storage_quota_mb"])

	// Default quota is 500 MB; the course now has 1 MB.
	_, q := flJSON(t, teacher, http.MethodGet, fmt.Sprintf("/courses/%d/files/quota", flCourse), nil)
	assert.EqualValues(t, 1024*1024, q["quota"])
	_, q = flJSON(t, teacher, http.MethodGet, fmt.Sprintf("/courses/%d/files/quota", flOther), nil)
	assert.EqualValues(t, 500*1024*1024, q["quota"])

	big := string(bytes.Repeat([]byte("a"), 700*1024))
	resp, first := flUpload(t, teacher, flCourse, "first.txt", big, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	resp, _ = flUpload(t, teacher, flCourse, "second.txt", big, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "over quota")

	_, q = flJSON(t, teacher, http.MethodGet, fmt.Sprintf("/courses/%d/files/quota", flCourse), nil)
	assert.EqualValues(t, 700*1024, q["quota_used"])

	// Replacing a file only counts the difference.
	resp, _ = flUpload(t, teacher, flCourse, "first.txt", big+"more", map[string]string{"_replace": fmt.Sprint(first["id"])})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// null resets to the account default.
	resp, out = flJSON(t, admin, http.MethodPut, fmt.Sprintf("/courses/%d", flCourse), map[string]any{"course": map[string]any{"storage_quota_mb": nil}})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, out["storage_quota_mb"])
}

// A designer sees unpublished files and folders in the course listing, so
// the routes without course middleware (/files/:id/download, /folders/:id)
// must let them through too, not answer 404 as they do for learners.
func TestFileVisibility_DesignerReachesUnpublishedByLink(t *testing.T) {
	f := newFilesLaneFixture(t)
	const designerID = uint(43)
	require.NoError(t, postgres.NewUserRepository(f.g).Create(context.Background(), &models.User{
		ID: designerID, AccountID: 1, Name: "Dana Designer", LoginID: "dana.fl@example.test", Email: "dana.fl@example.test", PasswordHash: "x", Role: "user",
	}))
	require.NoError(t, f.g.Create(&models.Enrollment{UserID: designerID, CourseID: flCourse, Type: "DesignerEnrollment", Role: "DesignerEnrollment", WorkflowState: models.EnrollmentActive}).Error)

	teacher := f.app(flTeacher, "TeacherEnrollment")
	designer := f.app(designerID, "DesignerEnrollment")
	student := f.app(flStudent, "StudentEnrollment")

	_, key := flUpload(t, teacher, flCourse, "answer-key.txt", "secret", nil)
	resp, _ := flJSON(t, teacher, http.MethodPut, fmt.Sprintf("/courses/%d/files/%v", flCourse, key["id"]), map[string]any{"locked": true})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp, folder := flJSON(t, teacher, http.MethodPost, fmt.Sprintf("/courses/%d/folders", flCourse), map[string]any{"name": "Drafts", "locked": true})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	assert.Contains(t, flNames(t, testutil.MakeRequest(designer, http.MethodGet, fmt.Sprintf("/courses/%d/files", flCourse), nil)), "answer-key.txt")
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(designer, http.MethodGet, fmt.Sprintf("/files/%v/download", key["id"]), nil).StatusCode)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(designer, http.MethodGet, fmt.Sprintf("/folders/%v", folder["id"]), nil).StatusCode)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(designer, http.MethodGet, fmt.Sprintf("/folders/%v/files", folder["id"]), nil).StatusCode)

	// Learners are still kept out.
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/files/%v/download", key["id"]), nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, testutil.MakeRequest(student, http.MethodGet, fmt.Sprintf("/folders/%v", folder["id"]), nil).StatusCode)
}
